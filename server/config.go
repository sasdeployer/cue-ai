package main

import (
	"log"
	"os"
	"strconv"
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
	// SharedTokenCap bounds how many tokens visitors may spend on the server's
	// OWN key before Cue stops serving them and asks them to bring their own
	// (BYOK). It exists so a public deployment can't run up an unbounded bill.
	// Set SHARED_TOKEN_CAP=0 to disable the cap entirely — correct for a local
	// or private deployment where the only person spending the key is you.
	SharedTokenCap int64
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
		SharedTokenCap:  envInt64("SHARED_TOKEN_CAP", 1_000_000),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envInt64 reads an integer env var, falling back to def when unset or
// unparseable. A malformed value must not silently become an uncapped 0.
func envInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Printf("config: %s=%q is not a number, using %d", key, v, def)
		return def
	}
	return n
}
