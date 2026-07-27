package main

import (
	"os"
)

// Config holds runtime configuration read from the environment.
type Config struct {
	Port            string
	DatabaseURL     string
	AnthropicKey    string
	AnthropicModel  string
	OpenAIKey       string
	OpenAIModel     string
	OpenAIReasoning string
	AllowOrigin     string
	// StaticDir is the built frontend (web/dist). Empty/missing in local dev,
	// where Vite serves the app on :5273 instead — see dev.sh.
	StaticDir string
}

func loadConfig() Config {
	return Config{
		Port:            env("PORT", "8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://cueai:cueai@localhost:5432/cueai?sslmode=disable"),
		AnthropicKey:    os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:  env("ANTHROPIC_MODEL", "claude-sonnet-5"),
		OpenAIKey:       os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:     env("OPENAI_MODEL", "gpt-5.2"),
		OpenAIReasoning: env("OPENAI_REASONING_EFFORT", "medium"),
		AllowOrigin:     env("ALLOW_ORIGIN", "http://localhost:5273"),
		StaticDir:       env("STATIC_DIR", "./dist"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
