package config

import (
	"fmt"
	"net"
)

type Config struct {
	ServerAddr string
}

func NewDefaultConfig() *Config {
	return &Config{
		ServerAddr: "127.0.0.1:8080",
	}
}

func (c *Config) Validate() error {
	// Checks the format without resolving the hostname
	host, port, err := net.SplitHostPort(c.ServerAddr)
	if err != nil {
		return err
	}
	if host == "" {
		return fmt.Errorf("missing host in address %q", c.ServerAddr)
	}
	if port == "" {
		return fmt.Errorf("missing port in address %q", c.ServerAddr)
	}
	return nil
}
