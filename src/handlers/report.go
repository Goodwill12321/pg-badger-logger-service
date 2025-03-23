package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"pg-badger-service/src/config"

	"github.com/gin-gonic/gin"
)

type ReportProcess struct {
	Cmd       []*exec.Cmd
	StartTime time.Time
}

var reportProcesses sync.Map

/*func getOSCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/C", "\""+command+"\"")
		return cmd
	} else {
		cmd := exec.Command("/bin/sh", "-c", command)
		return cmd
	}
}*/

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
	//reportKey := filepath.Join(serverReportDir, logFile)
	outputPath := filepath.Join(serverReportDir, strings.TrimSuffix(logFile, ".log")+".out")

	// Check if report is already being generated
	if _, exists := reportProcesses.Load(reportPath); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Report generation already in progress"})
		return
	}

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create output file: %v", err)})
		return
	}

	// Create psql command to fetch log content
	psqlCmd := exec.Command("psql", "-A", "-q", "-t",
		"-h", server.Host,
		"-p", fmt.Sprintf("%d", server.Port),
		"-U", server.User,
		"-d", server.Database,
		"-c", fmt.Sprintf("SELECT pg_read_file('pg_log/%s');", logFile))

	fmt.Println(psqlCmd.String())
	pgbadgerCmd := exec.Command("perl", "pgbadger", "-f", "stderr", "-v", "-o", reportPath, "-")

	pgbadgerCmd.Stdout = outputFile
	pgbadgerCmd.Stderr = outputFile

	psqlCmd.Stderr = outputFile

	pipe, _ := psqlCmd.StdoutPipe()
	pgbadgerCmd.Stdin = pipe

	fmt.Println(pgbadgerCmd.String())

	var processes []*exec.Cmd
	processes = append(processes, psqlCmd, pgbadgerCmd)
	// Store the process
	reportProcesses.Store(reportPath, &ReportProcess{
		Cmd:       processes,
		StartTime: time.Now(),
	})

	if err := pgbadgerCmd.Start(); err != nil {
		outputFile.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start pgbadger: %v", err)})
		return
	}

	go func() {
		defer outputFile.Close()
		defer reportProcesses.Delete(reportPath)
		if err := psqlCmd.Start(); err != nil {
			outputFile.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start psql: %v", err)})
			return
		}
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

	reportKey := strings.TrimSuffix(reportPath, ".log") + ".html"
	processInterface, exists := reportProcesses.Load(reportKey)
	if !exists {
		printActiveReports()
		c.JSON(http.StatusNotFound, gin.H{"error": "No active report generation found"})
		return
	}

	repProc := processInterface.(*ReportProcess)
	for _, proc := range repProc.Cmd {
		if err := killProcessTree(proc.Process.Pid); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to stop proc %d report generation: %v", proc.Process.Pid, err)})
			return
		}
	}

	reportProcesses.Delete(reportKey)
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

func printActiveReports() {
	reportProcesses.Range(func(key, value interface{}) bool {
		fmt.Printf("Key: %v, Value: %v\n", key, value)
		return true
	})
}

func killProcessTree(pid int) error {
	if runtime.GOOS == "windows" {
		// Windows: taskkill /T /F /PID <pid>
		cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Run()
	} else {
		// Linux/macOS: рекурсивное завершение процессов
		return killProcessTreeUnix(pid)
	}
}

// killProcessTreeUnix завершает процесс и его дочерние процессы (Linux/macOS)
func killProcessTreeUnix(pid int) error {
	// Получаем дочерние процессы
	childPIDs, err := findChildProcesses(pid)
	if err != nil {
		return err
	}

	// Убиваем дочерние процессы
	for _, childPID := range childPIDs {
		_ = killProcessTreeUnix(childPID) // Рекурсия
	}

	// Убиваем сам процесс
	process, err := os.FindProcess(pid)
	if err == nil {
		return process.Kill()
	}
	return err
}

// findChildProcesses ищет дочерние процессы (Linux/macOS)
func findChildProcesses(pid int) ([]int, error) {
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var childPIDs []int
	for _, line := range output {
		childPID, _ := strconv.Atoi(string(line))
		childPIDs = append(childPIDs, childPID)
	}

	return childPIDs, nil
}
