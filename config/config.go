package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Config represents all configuration values
type Config struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	Website     string   `json:"website"`
	Repository  string   `json:"repository"`
	Port        string   `json:"port" env:"PORT"`
	TrackingID  string   `json:"tracking_id"`
	Database    struct {
		DbUser string
		DbPass string
		DbHost string
	}
	Slack struct {
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

	port := os.Getenv("PORT")
	if len(port) > 1 {
		log.Println("Port override!")
		config.Port = port
	}

	config.Database.DbHost = os.Getenv("DB_HOST")
	config.Database.DbUser = os.Getenv("DB_USER")
	config.Database.DbPass = os.Getenv("DB_PASS")

	cfg = &config

	return cfg
}
