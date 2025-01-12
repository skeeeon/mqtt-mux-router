// File: config/config.go
package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	MQTT MQTTConfig `json:"mqtt"`
}

type MQTTConfig struct {
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	TLS      struct {
		Enable   bool   `json:"enable"`
		CertFile string `json:"certFile"`
		KeyFile  string `json:"keyFile"`
		CAFile   string `json:"caFile"`
	} `json:"tls"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
