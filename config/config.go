package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	SMTPConfig         SMTPConfig `yaml:"smtp"`
	ExcludedNamespaces []string   `yaml:"excluded_namespaces"`
	LogTailLines       int        `yaml:"log_tail_lines"`
}

type SMTPConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	From   string `yaml:"from"`
	NoAuth bool   `yaml:"no_auth"`
}

func Load(configPath string) (*Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.LogTailLines == 0 {
		cfg.LogTailLines = 50
	}

	return &cfg, nil
}
