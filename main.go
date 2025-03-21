package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"pg-badger-service/src/config"
	"pg-badger-service/src/handlers"
)

/* var (
	reportLock sync.Map // Map to track running reports
) */

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	// Load configuration
	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("%v", err)
	}

	router := gin.Default()

	// Serve static files
	router.Static("/static", "./static")
	router.Static("/report", "./report")

	router.LoadHTMLGlob("templates/*")

	/*
		dir, _ := os.Getwd()
		fmt.Println("Current dir:", dir) */

	// API routes
	router.GET("/", handleIndex)
	router.GET("/api/servers", handlers.GetServers)
	router.GET("/api/logs/:server", handleGetLogs)
	router.POST("/api/report/:server", handleGenerateReport)
	router.GET("/api/reports/:server", handleGetReports)
	router.GET("/api/report-status/:server/:report", handleReportStatus)
	router.POST("/api/stop-report/:server/:report", handleStopReport)

	thisServicePort := config.GetThisServicePort()

	fmt.Println("Server starting on http://localhost:", thisServicePort)
	if err := router.Run(fmt.Sprintf(":%d", thisServicePort)); err != nil {
		log.Fatalf("error starting server: %v", err)
	}
}

func handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"servers": config.GetServers(),
	})
}

func handleGetLogs(c *gin.Context) {
	serverName := c.Param("server")
	server, found := config.GetServerByName(serverName)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	handlers.GetLogs(c, server)
}

func handleGenerateReport(c *gin.Context) {
	serverName := c.Param("server")
	handlers.GenerateReport(c, serverName, config.GetReportDir())
}

func handleGetReports(c *gin.Context) {
	serverName := c.Param("server")
	handlers.GetReports(c, serverName, config.GetReportDir())
}

func handleReportStatus(c *gin.Context) {
	serverName := c.Param("server")
	reportName := c.Param("report")
	handlers.GetReportStatus(c, serverName, reportName, config.GetReportDir())
}

func handleStopReport(c *gin.Context) {
	serverName := c.Param("server")
	reportName := c.Param("report")
	handlers.StopReport(c, serverName, reportName, config.GetReportDir())
}
