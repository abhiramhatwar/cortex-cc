package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DBPath      string
	OllamaURL   string
	OllamaModel string
	WhisperURL  string
	HFToken     string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DBPath:      getEnv("DB_PATH", "./cortex.db"),
		OllamaURL:   getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel: getEnv("OLLAMA_MODEL", "llama3.1:8b"),
		WhisperURL:  getEnv("WHISPER_URL", "http://localhost:8001"),
		HFToken:     getEnv("HF_TOKEN", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
