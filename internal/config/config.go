package config

import "os"

type Config struct {
	Server ServerConfig
}

type ServerConfig struct {
	Addr      string
	PublicURL string
	StaticDir string
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Addr:      getenv("IBP_SERVER_ADDR", "0.0.0.0:8080"),
			PublicURL: getenv("IBP_PUBLIC_URL", "http://localhost:8080"),
			StaticDir: getenv("IBP_STATIC_DIR", "web/dist"),
		},
	}
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
