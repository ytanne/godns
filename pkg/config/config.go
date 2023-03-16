package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

//nolint:tagliatelle
type Config struct {
	WebConfig WebServerConfig `yaml:"web-config"`
	DnsPort   string          `yaml:"dns-port"`
	DbPath    string          `yaml:"db-path"`
}

//nolint:tagliatelle
type WebServerConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	HttpPort string `yaml:"http-port"`
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
