package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

type Report struct {
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"createdAt"`
	IsProcessing bool      `json:"isProcessing"`
}

func GetReports(c *gin.Context, serverName string, reportDir string) {
	serverReportDir := filepath.Join(reportDir, serverName)

	entries, err := os.ReadDir(serverReportDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, []Report{})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read reports directory"})
		return
	}

	var reports []Report = []Report{} // Initialize empty array to return not null

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".html" {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			reportPath := filepath.Join(serverReportDir, entry.Name())
			_, isProcessing := reportProcesses.Load(reportPath)

			reports = append(reports, Report{
				Name:         entry.Name(),
				CreatedAt:    info.ModTime(),
				IsProcessing: isProcessing,
			})
		}
	}

	c.JSON(http.StatusOK, reports)
}
