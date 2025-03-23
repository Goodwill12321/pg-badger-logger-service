package handlers

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"pg-badger-service/src/config"

	"github.com/gin-gonic/gin"
)

type ReportProcess struct {
	Cmd       *exec.Cmd
	StartTime time.Time
}

var reportProcesses sync.Map

func getOSCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/C", command)
		return cmd
	} else {
		cmd := exec.Command("/bin/sh", "-c", command)
		return cmd
	}
}

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
	/*psqlCmd := exec.Command("psql", "-A", "-q",
	"-h", server.Host,
	"-p", fmt.Sprintf("%d", server.Port),
	"-U", server.User,
	"-d", server.Database,
	"-c", fmt.Sprintf("\"SELECT pg_read_file('pg_log/%s', 0, 1000000000000);\"", logFile))*/

	psqlCmd := getOSCommand(fmt.Sprintf("psql -A -q -h %s -p %d -U %s -d %s -c \"SELECT pg_read_file('pg_log/%s');\"",
		server.Host,
		server.Port, server.User,
		server.Database,
		logFile))

	fmt.Println(psqlCmd.String())
	// Create pgbadger command
	//pgbadgerCmd := exec.Command("perl", "pgbadger", "-f", "stderr", "-v", "-o", reportPath, "-")
	pgbadgerCmd := getOSCommand(fmt.Sprintf("perl pgbadger -f stderr -v -o %s - ", reportPath))

	fmt.Println(pgbadgerCmd.String())

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create output file: %v", err)})
		return
	}

	// Create pipes to connect the commands
	r, w := io.Pipe()

	// Set up pgbadger command to read from the pipe
	pgbadgerCmd.Stdin = r
	pgbadgerCmd.Stdout = outputFile
	pgbadgerCmd.Stderr = outputFile

	// Start pgbadger
	if err := pgbadgerCmd.Start(); err != nil {
		r.Close()
		w.Close()
		outputFile.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start pgbadger: %v", err)})
		return
	}

	// Store the process
	reportProcesses.Store(reportPath, &ReportProcess{
		Cmd:       pgbadgerCmd,
		StartTime: time.Now(),
	})

	// Run psql and process its output in a goroutine
	go func() {
		defer w.Close()
		defer outputFile.Close()
		defer reportProcesses.Delete(reportPath)

		// Set up stdout pipe for psql
		stdout, err := psqlCmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(outputFile, "Error creating stdout pipe: %v\n", err)
			return
		}

		// Start the psql command
		if err := psqlCmd.Start(); err != nil {
			fmt.Fprintf(outputFile, "Error starting psql command: %v\n", err)
			return
		}

		// Process the output line by line
		scanner := bufio.NewScanner(stdout)
		var lineCount int
		var isFirstLine = true

		// Read and process each line
		for scanner.Scan() {
			lineCount++
			text := scanner.Text()

			// Skip the first line
			if isFirstLine {
				isFirstLine = false
				continue
			}

			// Skip the last line that starts with '(' and ends with ')'
			if len(text) > 0 && text[0] == '(' && text[len(text)-1] == ')' {
				continue
			}

			// Write the line to the pipe
			fmt.Fprintln(w, text)
		}

		// Check for scanning errors
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(outputFile, "Error reading psql output: %v\n", err)
		}

		/*// Wait for psql to finish
		if err := psqlCmd.Wait(); err != nil {
			fmt.Fprintf(outputFile, "Error waiting for psql command: %v\n", err)
		}*/

		// Wait for pgbadger to finish
		if err := pgbadgerCmd.Wait(); err != nil {
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
