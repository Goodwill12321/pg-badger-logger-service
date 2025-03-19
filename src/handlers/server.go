package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"pg-badger-service/src/config"
)

// GetServers returns the list of configured PostgreSQL servers
func GetServers(c *gin.Context) {
	c.JSON(http.StatusOK, config.GetServers())
}
