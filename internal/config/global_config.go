package config

import (
	"os"
	"strings"

	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

const (
	// EnvLocalGeneratorType is the environment variable for the local generator type
	EnvLocalGeneratorType = "FLOWCTL_LOCAL_GENERATOR_TYPE"
	
	// LocalGeneratorDocker is the Docker Compose generator type
	LocalGeneratorDocker = "docker"
	
	// LocalGeneratorBash is the bash script generator type
	LocalGeneratorBash = "bash"
)

// GlobalConfig contains application-wide configuration
type GlobalConfig struct {
	// LocalGeneratorType determines which local generator implementation to use
	LocalGeneratorType string
}

// DefaultGlobalConfig returns the default global configuration
func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		LocalGeneratorType: LocalGeneratorDocker, // Default to Docker Compose
	}
}

// LoadGlobalConfig loads the global configuration from environment variables
func LoadGlobalConfig() GlobalConfig {
	cfg := DefaultGlobalConfig()
	
	// Load local generator type from environment
	if genType := os.Getenv(EnvLocalGeneratorType); genType != "" {
		genType = strings.ToLower(genType)
		if genType == LocalGeneratorBash || genType == LocalGeneratorDocker {
			cfg.LocalGeneratorType = genType
			logger.Debug("Using local generator type from environment", 
				zap.String("type", genType))
		} else {
			logger.Warn("Unknown local generator type in environment, using default",
				zap.String("type", genType),
				zap.String("default", cfg.LocalGeneratorType))
		}
	}
	
	return cfg
}

// UseDockerComposeGenerator returns true if the Docker Compose generator should be used
func (c GlobalConfig) UseDockerComposeGenerator() bool {
	return c.LocalGeneratorType == LocalGeneratorDocker
}

// UseBashScriptGenerator returns true if the bash script generator should be used
func (c GlobalConfig) UseBashScriptGenerator() bool {
	return c.LocalGeneratorType == LocalGeneratorBash
}