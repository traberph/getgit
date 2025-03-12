package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDirName  = "getgit"
	SourcesDirName = "sources.d"
)

type Config struct {
	Root string `yaml:"root"`
}

// GetConfigDir returns the path to the getgit config directory
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", ConfigDirName), nil
}

// GetSourcesDir returns the path to the sources.d directory
func GetSourcesDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, SourcesDirName), nil
}

// GetWorkDir returns the path to the work directory
func GetWorkDir() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return cfg.Root, nil
}

// GetCacheDir returns the path to the getgit cache directory
func GetCacheDir() (string, error) {
	// First check XDG_CACHE_HOME
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		// If not set, use default ~/.cache
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cacheHome = filepath.Join(homeDir, ".cache")
	}
	return filepath.Join(cacheHome, ConfigDirName), nil
}

// GetAliasFile returns the path to the alias file
func GetAliasFile() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return filepath.Join(cfg.Root, ".alias"), nil
}

// LoadConfig loads the configuration from the config file
// If the config file doesn't exist, it creates a default one
func LoadConfig() (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// Config file doesn't exist, create a default one
		// Get the current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		// Use parent directory as root since that's where the tool lives
		defaultRoot := filepath.Dir(cwd)
		config := &Config{
			Root: defaultRoot,
		}
		data, err := yaml.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default config: %w", err)
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write default config: %w", err)
		}
		return config, nil
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
