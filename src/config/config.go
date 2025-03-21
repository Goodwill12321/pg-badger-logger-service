package config

import (
	"fmt"
	"os"
	"path/filepath"

	"pg-badger-service/src/models"

	"gopkg.in/yaml.v2"
)

// Config represents the application configuration
type Config struct {
	ThisServicePort int                     `yaml:"this_service_port"`
	Servers         []models.PostgresServer `yaml:"servers"`
	ReportDir       string                  `yaml:"report_dir"`
}

// AppConfig is the global configuration instance
var AppConfig Config

// LoadConfig loads the configuration from a YAML file
func LoadConfig(configPath string) error {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	if err := yaml.Unmarshal(data, &AppConfig); err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}

	// Create report directory if it doesn't exist
	if err := os.MkdirAll(AppConfig.ReportDir, 0755); err != nil {
		return fmt.Errorf("error creating report directory: %v", err)
	}

	// Create server-specific report directories
	for _, server := range AppConfig.Servers {
		serverReportDir := filepath.Join(AppConfig.ReportDir, server.Name)
		if err := os.MkdirAll(serverReportDir, 0755); err != nil {
			return fmt.Errorf("error creating server report directory: %v", err)
		}
	}

	return nil
}

// GetServers returns the list of configured PostgreSQL servers
func GetServers() []models.PostgresServer {
	return AppConfig.Servers
}

// GetServerByName returns a server by its name
func GetServerByName(name string) (models.PostgresServer, bool) {
	for _, server := range AppConfig.Servers {
		if server.Name == name {
			return server, true
		}
	}
	return models.PostgresServer{}, false
}

// GetReportDir returns the report directory path
func GetReportDir() string {
	return AppConfig.ReportDir
}

// GetReportDir returns the report directory path
func GetThisServicePort() int {
	return AppConfig.ThisServicePort
}
