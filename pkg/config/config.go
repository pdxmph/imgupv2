package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	Flickr FlickrConfig `json:"flickr"`
}

// FlickrConfig holds Flickr-specific configuration
type FlickrConfig struct {
	ConsumerKey    string `json:"consumer_key"`
	ConsumerSecret string `json:"consumer_secret"`
	AccessToken    string `json:"access_token,omitempty"`
	AccessSecret   string `json:"access_secret,omitempty"`
}

// Load loads configuration from the default location
func Load() (*Config, error) {
	path := configPath()
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	return &cfg, nil
}

// Save saves the configuration
func (c *Config) Save() error {
	path := configPath()
	
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	
	return nil
}

// configPath returns the configuration file path
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "imgupv2", "config.json")
}
