package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var ctx = context.Background()
var rdb *redis.Client

func main() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	r := gin.Default()

	r.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		userJSON, err := rdb.Get(ctx, id).Result()
		if err == redis.Nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		} else {
			var user User
			json.Unmarshal([]byte(userJSON), &user)
			c.JSON(http.StatusOK, user)
		}
	})

	r.POST("/users", func(c *gin.Context) {
		var newUser User
		if err := c.BindJSON(&newUser); err == nil {
			userJSON, _ := json.Marshal(newUser)
			err := rdb.Set(ctx, newUser.ID, userJSON, 0).Err()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			} else {
				c.JSON(http.StatusCreated, newUser)
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
	})

	r.PUT("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		userJSON, err := rdb.Get(ctx, id).Result()
		if err == redis.Nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		} else {
			var user User
			json.Unmarshal([]byte(userJSON), &user)

			var updatedUser User
			if err := c.BindJSON(&updatedUser); err == nil {
				user.Name = updatedUser.Name
				userJSON, _ := json.Marshal(user)
				err := rdb.Set(ctx, user.ID, userJSON, 0).Err()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				} else {
					c.JSON(http.StatusOK, user)
				}
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			}
		}
	})

	r.DELETE("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		err := rdb.Del(ctx, id).Err()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
		}
	})

	r.Run(":8080")
}
