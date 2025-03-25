package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"pg-badger-service/src/models"
)

type LogFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Date string `json:"date"`
}

func GetLogs(c *gin.Context, server models.PostgresServer) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=%s",
		server.Host, server.Port, server.User, server.Password, server.SSLMode)

	os.Unsetenv("PGLOCALEDIR")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to database: %v", err)})
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT * FROM pg_ls_logdir() WHERE name like '%.log' ORDER BY modification")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get logs: %v", err)})
		return
	}
	defer rows.Close()

	var logs []LogFile
	for rows.Next() {
		var log LogFile
		if err := rows.Scan(&log.Name, &log.Size, &log.Date); err != nil {
			fmt.Printf("Error scanning log row: %v\n", err)
			continue
		}
		logs = append(logs, log)
	}

	c.JSON(http.StatusOK, logs)
}
