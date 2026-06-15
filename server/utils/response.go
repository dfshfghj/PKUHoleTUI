package utils

import (
	"log"

	"github.com/gin-gonic/gin"
)

func RespondError(c *gin.Context, code int, errid string, err error) {
	if err != nil {
		log.Printf("[API] %s: %v", errid, err)
	}
	c.JSON(code, gin.H{
		"status":  code,
		"errid":   errid,
		"message": "An internal error occurred",
	})
}

func RespondSuccess(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{
		"status": 200,
		"data":   data,
	})
}
