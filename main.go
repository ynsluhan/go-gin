package main

import (
	"github.com/gin-gonic/gin"
	e "github.com/ynsluhan/go-gin/engine"
)

func main() {
	engine := e.NewEngine()
	engine.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200})
		return
	})
	e.Run(engine)
}
