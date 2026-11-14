package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/razenry/mcqueen-ai/internal/ai"
	"github.com/razenry/mcqueen-ai/internal/config"
	"github.com/razenry/mcqueen-ai/internal/discord"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Load character personality.
	prompt, err := os.ReadFile("prompts/mcqueen.txt")
	if err != nil {
		log.Fatalf("Failed to load character prompt: %v", err)
	}

	// Initialize AI provider.
	provider, err := ai.NewRouter(ai.RouterConfig{
		BaseURL:     cfg.AIBaseURL,
		APIKey:      cfg.AIAPIKey,
		Model:       cfg.AIModel,
		Temperature: cfg.AITemperature,
		MaxTokens:   cfg.AIMaxTokens,
		Timeout:     cfg.AITimeout,
	})
	if err != nil {
		log.Fatalf("AI initialization failed: %v", err)
	}

	// Initialize Discord bot.
	bot, err := discord.New(
		cfg.DiscordToken,
		provider,
		string(prompt),
	)
	if err != nil {
		log.Fatalf("Discord initialization failed: %v", err)
	}

	if err := bot.Start(); err != nil {
		log.Fatalf("Discord connection failed: %v", err)
	}

	log.Printf("%s is running!", cfg.AppName)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	<-ctx.Done()

	log.Println("Shutting down...")

	if err := bot.Close(); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
