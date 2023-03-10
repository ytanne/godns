package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

//nolint:tagliatelle
type Config struct {
	DnsPort   string `yaml:"dns-port"`
	HttpPort  string `yaml:"http-port"`
	SecretKey string `yaml:"secret-key"`
	DbPath    string `yaml:"db-path"`
}

func NewConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	c := new(Config)
	err = yaml.Unmarshal(data, c)

	return *c, err
}
