package config

import (
	"fmt"
	"net"
)

type Config struct {
	ServerAddr string `yaml:"server_addr"`
	BaseURL    string `yaml:"base_url"`
}

func NewDefaultConfig() *Config {

	return &Config{
		ServerAddr: "localhost:8080",
	}
}

func (c *Config) Validate() error {
	// Checks the format without resolving the hostname
	host, port, err := net.SplitHostPort(c.ServerAddr)
	if err != nil {
		return fmt.Errorf("error parsing server address: %w", err)
	}
	if host == "" {
		return fmt.Errorf("missing host in address %q", c.ServerAddr)
	}
	if port == "" {
		return fmt.Errorf("missing port in address %q", c.ServerAddr)
	}
	return nil
}
