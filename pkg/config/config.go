package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	Flickr    FlickrConfig          `json:"flickr"`
	Mastodon  MastodonConfig        `json:"mastodon"`
	Templates map[string]string     `json:"templates,omitempty"`
}

// FlickrConfig holds Flickr-specific configuration
type FlickrConfig struct {
	ConsumerKey    string `json:"consumer_key"`
	ConsumerSecret string `json:"consumer_secret"`
	AccessToken    string `json:"access_token,omitempty"`
	AccessSecret   string `json:"access_secret,omitempty"`
}

// MastodonConfig holds Mastodon-specific configuration
type MastodonConfig struct {
	InstanceURL  string `json:"instance_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AccessToken  string `json:"access_token,omitempty"`
}

// DefaultTemplates returns the default output templates
func DefaultTemplates() map[string]string {
	return map[string]string{
		"markdown": "![%alt%|%description%|%title%|%filename%](%image_url%)",
		"html":     `<img src="%image_url%" alt="%alt%|%description%|%title%|%filename%">`,
		"url":      "%url%",
		"json":     `{"photo_id":"%photo_id%","url":"%url%","image_url":"%image_url%"}`,
	}
}

// Load loads configuration from the default location
func Load() (*Config, error) {
	path := configPath()
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return config with default templates
			return &Config{
				Templates: DefaultTemplates(),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	// Ensure default templates exist
	if cfg.Templates == nil {
		cfg.Templates = DefaultTemplates()
	} else {
		// Add any missing default templates
		defaults := DefaultTemplates()
		for k, v := range defaults {
			if _, exists := cfg.Templates[k]; !exists {
				cfg.Templates[k] = v
			}
		}
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
