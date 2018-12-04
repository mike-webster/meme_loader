package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents all configuration values
type Config struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	Website     string   `json:"website"`
	Repository  string   `json:"repository"`
	Port        int      `json:"port" env:"PORT"`
	Slack       struct {
		WebHook string `json:"webhook"`
	}
}

var cfg *Config

// GetConfig will load the configuration from the given path unless we already have a cached value. If it is loaded, it is then cached
func GetConfig(file string) *Config {
	if cfg != nil {
		return cfg
	}

	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()

	if err != nil {
		fmt.Println(err.Error())
	}

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	cfg = &config

	return cfg
}
