package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Rules      RulesConfig      `yaml:"rules"`
}

type ServerConfig struct {
	Port           int `yaml:"port"`
	MaxConcurrency int `yaml:"max_concurrency"`
}

type PrometheusConfig struct {
	URL string `yaml:"url"`
}

type RulesConfig struct {
	AllowedSpaces []string `yaml:"allowed_spaces"`
}

func LoadFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
