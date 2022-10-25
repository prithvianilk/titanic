package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	const addr = ":3530"
	kvmap := make(map[string]string)
	router := gin.Default()
	router.GET("/:key", func(c *gin.Context) {
		key := c.Param("key")
		value := kvmap[key]
		c.String(http.StatusOK, value)
	})
	router.PUT("/:key/:value", func(c *gin.Context) {
		key, value := c.Param("key"), c.Param("value")
		kvmap[key] = value
		c.String(http.StatusOK, "")
	})
	router.Run(addr)
}
