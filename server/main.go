package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// load ../.env then ./.env (either location works)
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := loadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	store, err := NewStore(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()

	var llm LLMClient
	switch {
	case cfg.OpenAIKey != "":
		llm = NewOpenAIClient(cfg.OpenAIKey, cfg.OpenAIModel, cfg.OpenAIReasoning)
		log.Printf("LLM: %s", llm.Name())
	case cfg.AnthropicKey != "":
		llm = NewAnthropicClient(cfg.AnthropicKey, cfg.AnthropicModel)
		log.Printf("LLM: %s", llm.Name())
	default:
		llm = &CannedClient{}
		log.Printf("LLM: canned (no OPENAI_API_KEY / ANTHROPIC_API_KEY set — decks are sample decks)")
	}

	api := &API{store: store, llm: llm, cfg: cfg}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{cfg.AllowOrigin},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		// X-User-*-Key: optional bring-your-own-key override, read per-request
		// and never persisted server-side (see handlers.go's llmFor).
		AllowHeaders: []string{"Content-Type", "X-User-OpenAI-Key", "X-User-Anthropic-Key"},
	}))

	r.GET("/api/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true, "llm": llm.Name()}) })
	r.POST("/api/decks", api.generate)
	r.GET("/api/decks", api.list)
	r.GET("/api/decks/:id", api.get)
	r.POST("/api/decks/:id/edit", api.editDeck)

	// In production (single container) this same process serves the built
	// frontend; in local dev the directory doesn't exist and Vite serves it.
	// Real files in dist win — assets/, hero/, and the deck-runtime.html
	// second Vite entry — everything else falls through to index.html so
	// TanStack Router's client-side routes survive a hard refresh.
	if st, err := os.Stat(cfg.StaticDir); err == nil && st.IsDir() {
		log.Printf("serving frontend from %s", cfg.StaticDir)
		index := filepath.Join(cfg.StaticDir, "index.html")
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			// Clean against "/" first so ".." can't escape StaticDir.
			f := filepath.Join(cfg.StaticDir, filepath.Clean("/"+c.Request.URL.Path))
			if st, err := os.Stat(f); err == nil && !st.IsDir() {
				c.File(f)
				return
			}
			c.File(index)
		})
	} else {
		log.Printf("no static dir at %s — API only (Vite serves the frontend in dev)", cfg.StaticDir)
	}

	log.Printf("Cue AI server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
