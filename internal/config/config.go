package config

import "net/netip"

type Config struct {
	ServerAddr string
}

func NewDefaultConfig() *Config {
	return &Config{
		ServerAddr: "127.0.0.1:8080",
	}
}

func (c *Config) Validate() error {
	if _, err := netip.ParseAddrPort(c.ServerAddr); err != nil {
		return err
	}
	return nil
}
