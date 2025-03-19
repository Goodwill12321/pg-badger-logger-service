package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"pg-badger-service/src/config"
)

type ReportProcess struct {
	Cmd       *exec.Cmd
	StartTime time.Time
}

var reportProcesses sync.Map

func GenerateReport(c *gin.Context, serverName string, reportDir string) {
	logFile := c.PostForm("logFile")
	if logFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Log file not specified"})
		return
	}

	// Get server information from config
	server, found := config.GetServerByName(serverName)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	// Create report directory if it doesn't exist
	serverReportDir := filepath.Join(reportDir, serverName)
	if err := os.MkdirAll(serverReportDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create report directory: %v", err)})
		return
	}

	// Generate report filename
	reportName := strings.TrimSuffix(logFile, ".log") + ".html"
	reportPath := filepath.Join(serverReportDir, reportName)
	outputPath := filepath.Join(serverReportDir, strings.TrimSuffix(logFile, ".log")+".out")

	// Check if report is already being generated
	if _, exists := reportProcesses.Load(reportPath); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Report generation already in progress"})
		return
	}

	// Create psql command to fetch log content
	psqlCmd := fmt.Sprintf("psql -A -q -h %s -p %d -U %s -c \"SELECT pg_read_file('%s', 0, 1000000000000);\" | sed '1d;$d'", 
		server.Host, server.Port, server.User, logFile)

	// Create pgbadger command using psql output
	cmd := exec.Command("bash", "-c", fmt.Sprintf("%s | perl pgbadger -f stderr -v - -o %s", psqlCmd, reportPath))

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create output file: %v", err)})
		return
	}

	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	// Start the command
	if err := cmd.Start(); err != nil {
		outputFile.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start report generation: %v", err)})
		return
	}

	// Store the process
	reportProcesses.Store(reportPath, &ReportProcess{
		Cmd:       cmd,
		StartTime: time.Now(),
	})

	// Run the command in a goroutine
	go func() {
		defer outputFile.Close()
		defer reportProcesses.Delete(reportPath)

		if err := cmd.Wait(); err != nil {
			fmt.Printf("Error generating report: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Report generation started",
		"report":  reportName,
	})
}

func StopReport(c *gin.Context, serverName string, reportName string, reportDir string) {
	reportPath := filepath.Join(reportDir, serverName, reportName)

	processInterface, exists := reportProcesses.Load(reportPath)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active report generation found"})
		return
	}

	process := processInterface.(*ReportProcess)
	if err := process.Cmd.Process.Kill(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to stop report generation: %v", err)})
		return
	}

	reportProcesses.Delete(reportPath)
	c.JSON(http.StatusOK, gin.H{"message": "Report generation stopped"})
}

func GetReportStatus(c *gin.Context, serverName string, reportName string, reportDir string) {
	reportPath := filepath.Join(reportDir, serverName, reportName)
	outputPath := filepath.Join(reportDir, serverName, strings.TrimSuffix(reportName, ".html")+".out")

	processInterface, exists := reportProcesses.Load(reportPath)
	if !exists {
		// Check if report exists
		if _, err := os.Stat(reportPath); err == nil {
			c.JSON(http.StatusOK, gin.H{
				"status": "completed",
				"path":   reportPath,
			})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	process := processInterface.(*ReportProcess)
	output, err := os.ReadFile(outputPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read output file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "running",
		"startTime": process.StartTime,
		"output":    string(output),
	})
}
