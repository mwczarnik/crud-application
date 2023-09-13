package main

import (
	"context"
	"fmt"
	"sync"

	json "github.com/json-iterator/go"
	_ "go.uber.org/automaxprocs"

	"net/http"
	"os"

	// "github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

func main() {
	r := gin.Default()
	// pprof.Register(r)

	ctx := context.Background()
	client, _ := mongo.Connect(ctx,
		options.Client().ApplyURI(os.Getenv("MONGO_URI")).SetRetryWrites(true).SetRetryReads(true)) //.SetMinPoolSize(50).SetMaxConnecting(100)) //.SetCompressors([]string{"zstd"}).SetZstdLevel(10))

	collection := client.Database("crud_db").Collection("users")
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
		userPool.Put(user)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok", "id": res.InsertedID})
	})

	r.GET("/user/:id", func(c *gin.Context) {
		id := c.Param("id")
		user := userPool.Get().(*User)

		err := collection.FindOne(ctx, bson.M{"id": id}).Decode(&user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		json.NewEncoder(c.Writer).Encode(user)
		userPool.Put(user)
	})

	r.GET("/users", func(c *gin.Context) {
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)

		var usersPointer = usersPool.Get().(*[]User)
		var users = *usersPointer

		if err = cursor.All(c, &users); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		if err := cursor.Err(); err != nil {
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

		_, err := collection.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": bson.M{"name": user.Name}})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})

		userPool.Put(user)
	})

	r.DELETE("/user/:id", func(c *gin.Context) {
		id := c.Param("id")

		_, err := collection.DeleteOne(ctx, bson.M{"id": id})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.Run()
}
