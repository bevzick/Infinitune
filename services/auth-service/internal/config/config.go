package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

const (
	defaultHTTPHost = "0.0.0.0"
	defaultHTTPPort = "8080"
)

type Config struct {
	HTTPAddress string
}

func Load() (Config, error) {
	host := envOrDefault("AUTH_HTTP_HOST", defaultHTTPHost)
	port := envOrDefault("AUTH_HTTP_PORT", defaultHTTPPort)

	if err := validatePort(port); err != nil {
		return Config{}, err
	}

	return Config{HTTPAddress: net.JoinHostPort(host, port)}, nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}

func validatePort(port string) error {
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("invalid AUTH_HTTP_PORT %q: expected a number from 1 to 65535", port)
	}

	return nil
}
