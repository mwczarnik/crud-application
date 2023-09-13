package main

import (
	"context"
	"fmt"
	"sync"

	"net/http"
	"os"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"

	json "github.com/json-iterator/go"

	rejson "github.com/nitishm/go-rejson/v4"
	redis "github.com/redis/go-redis/v9"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	_ "go.uber.org/automaxprocs"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name" binding:"required"`
}

var userPool = sync.Pool{
	New: func() any {
		return &User{}
	},
}

var usersPool = sync.Pool{
	New: func() any {
		return &[]User{}
	},
}

func populateCache(ctx context.Context, users []User, redisClient *rejson.Handler) {
	for _, user := range users {
		redisClient.JSONSet(fmt.Sprintf("user:%s", user.ID), ".", user)
	}
}

func getUsers(ctx context.Context, collection *mongo.Collection) ([]User, error) {
	cursor, err := collection.Find(ctx, bson.M{})

	var users []User

	if err != nil {
		return nil, err
	}

	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func main() {

	r := gin.Default()
	pprof.Register(r)

	ctx := context.Background()
	client, _ := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	defer client.Disconnect(ctx)

	collection := client.Database("crud_db").Collection("users")

	redisDefaultClient := redis.NewClient(
		&redis.Options{
			Addr: os.Getenv("REDIS_HOST") + ":6379"})

	redisClient := rejson.NewReJSONHandler()
	redisClient.SetGoRedisClientWithContext(ctx, redisDefaultClient)

	dbUsers, err := getUsers(ctx, collection)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	populateCache(ctx, dbUsers, redisClient)

	indexModel := mongo.IndexModel{
		Keys: bson.M{
			"id": 1,
		},
		Options: options.Index().SetUnique(true),
	}

	if _, err := collection.Indexes().CreateOne(ctx, indexModel); err != nil {
		fmt.Println("Could not create index:", err)
	}

	// routes
	r.POST("/user", func(c *gin.Context) {
		user := userPool.Get().(*User)

		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		res, err := collection.InsertOne(ctx, bson.M{"id": user.ID, "name": user.Name})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		redisClient.JSONSet(fmt.Sprintf("user:%s", user.ID), ".", user)
		userPool.Put(user)

		c.JSON(http.StatusOK, gin.H{"status": "ok", "id": res.InsertedID})
	})

	r.GET("/user/:id", func(c *gin.Context) {
		id := c.Param("id")
		user := userPool.Get().(*User)
		userCached, _ := redisClient.JSONGet(fmt.Sprintf("user:%s", id), ".")

		if userCached != nil {
			if err := json.Unmarshal(userCached.([]byte), &user); err != nil {
				fmt.Println("Unable to Unmarshal object")
			}
		}

		if len(user.Name) > 0 {
			c.JSON(http.StatusOK, user)
			return
		}

		if err := collection.FindOne(ctx, bson.M{"id": id}).Decode(&user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		redisClient.JSONSet(fmt.Sprintf("user:%s", id), ".", user)

		c.JSON(http.StatusOK, user)

		userPool.Put(user)
	})

	r.GET("/users", func(c *gin.Context) {
		var usersPointer = usersPool.Get().(*[]User)
		var users = *usersPointer
		var user = userPool.Get().(*User)
		// var users []User
		// var user User

		//  Try using cached users
		// keys, err := redisClient.Keys(ctx, "user:*").Result()
		// if err != nil {
		// 	fmt.Println("No keys found in cache")
		// }

		keys, err := redisDefaultClient.Keys(ctx, "user:*").Result()
		if err != nil {
			fmt.Println("No keys found in cache")
		}

		usersCached, _ := redisClient.JSONMGet(".", keys...)

		for _, userBytes := range usersCached.([]interface{}) {
			json.Unmarshal(userBytes.([]byte), &user)
			users = append(users, *user)
		}
		userPool.Put(user)

		if len(users) > 0 {
			c.JSON(http.StatusOK, users)
			return
		}

		users, err = getUsers(ctx, collection)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, users)

		*usersPointer = users
		usersPool.Put(usersPointer)
	})

	r.PUT("/user/:id", func(c *gin.Context) {
		id := c.Param("id")

		var user = userPool.Get().(*User)

		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if _, err := collection.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": bson.M{"name": user.Name}}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		redisClient.JSONSet(fmt.Sprintf("user:%s", id), ".", user)

		c.JSON(http.StatusOK, gin.H{"status": "ok"})

		userPool.Put(user)
	})

	r.DELETE("/user/:id", func(c *gin.Context) {
		id := c.Param("id")

		if _, err := collection.DeleteOne(ctx, bson.M{"id": id}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		redisClient.JSONDel(fmt.Sprintf("user:%s", id), ".")

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.Run()
}
