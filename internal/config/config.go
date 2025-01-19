package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Telegram struct {
		Token         string `yaml:"token"`
		ChatID        string `yaml:"chat_id"`
		MessageThread int    `yaml:"message_thread"`
	} `yaml:"telegram"`
	Allora struct {
		RPC     string   `yaml:"rpc"`
		API     string   `yaml:"api"`
		Address []string `yaml:"address"`
	} `yaml:"allora"`
}

func Load() (*Config, error) {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
