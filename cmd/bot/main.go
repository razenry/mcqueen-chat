
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/razenry/mcqueen-ai/internal/agent"
	"github.com/razenry/mcqueen-ai/internal/ai"
	"github.com/razenry/mcqueen-ai/internal/config"
	"github.com/razenry/mcqueen-ai/internal/conversation"
	"github.com/razenry/mcqueen-ai/internal/database"
	"github.com/razenry/mcqueen-ai/internal/discord"
	"github.com/razenry/mcqueen-ai/internal/tools/websearch"
	usermemory "github.com/razenry/mcqueen-ai/internal/memory"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	log.Printf("Starting %s...", cfg.AppName)

	// 1. Run database migrations.
	migrationCtx, cancelMigration := context.WithTimeout(
		ctx,
		60*time.Second,
	)

	err = database.RunMigrations(
		migrationCtx,
		cfg.DatabaseURL,
	)

	cancelMigration()

	if err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	log.Println("Database migrations completed")

	// 2. Initialize PostgreSQL connection pool.
	dbCtx, cancelDB := context.WithTimeout(
		ctx,
		15*time.Second,
	)

	dbPool, err := database.NewPool(
		dbCtx,
		database.Config{
			URL:      cfg.DatabaseURL,
			MaxConns: cfg.DBMaxConns,
			MinConns: cfg.DBMinConns,
		},
	)

	cancelDB()

	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	defer dbPool.Close()

	log.Println("PostgreSQL connection pool initialized")

	// 3. Initialize persistent conversation manager.
	conversationManager := conversation.NewManager(
		dbPool,
		6,
	)

	log.Println("PostgreSQL conversation storage initialized")

	// 4. Load character personality.
	prompt, err := os.ReadFile("prompts/mcqueen.txt")
	if err != nil {
		return fmt.Errorf("failed to load character prompt: %w", err)
	}

	// 5. Initialize AI provider.
	router, err := ai.NewRouter(ai.RouterConfig{
		BaseURL:     cfg.AIBaseURL,
		APIKey:      cfg.AIAPIKey,
		Model:       cfg.AIModel,
		Temperature: cfg.AITemperature,
		MaxTokens:   cfg.AIMaxTokens,
		Timeout:     cfg.AITimeout,
	})
	if err != nil {
		return fmt.Errorf("AI initialization failed: %w", err)
	}

	// 6. Initialize persistent user memory.
	userMemory, err := usermemory.NewService(
		dbPool,
		router,
	)
	if err != nil {
		return fmt.Errorf("user memory initialization failed: %w", err)
	}

	log.Println("Persistent user memory initialized")

	// 7. Register available AI tools.
	tools := make([]agent.Tool, 0)

	webSearchEnabled, err := getBoolEnv(
		"WEB_SEARCH_ENABLED",
		false,
	)
	if err != nil {
		return fmt.Errorf("web search configuration error: %w", err)
	}

	if webSearchEnabled {
		searchTimeout, err := getDurationEnv(
			"TAVILY_TIMEOUT",
			15*time.Second,
		)
		if err != nil {
			return fmt.Errorf("invalid TAVILY_TIMEOUT: %w", err)
		}

		maxResults, err := getIntEnv(
			"TAVILY_MAX_RESULTS",
			5,
		)
		if err != nil {
			return fmt.Errorf("invalid TAVILY_MAX_RESULTS: %w", err)
		}

		searcher, err := websearch.NewTavilyClient(
			websearch.TavilyConfig{
				APIKey:      os.Getenv("TAVILY_API_KEY"),
				BaseURL:     getStringEnv("TAVILY_BASE_URL", "https://api.tavily.com"),
				MaxResults:  maxResults,
				SearchDepth: getStringEnv("TAVILY_SEARCH_DEPTH", "basic"),
				Topic:       getStringEnv("TAVILY_TOPIC", "general"),
				Timeout:     searchTimeout,
			},
		)
		if err != nil {
			return fmt.Errorf("Tavily initialization failed: %w", err)
		}

		searchTool, err := websearch.NewTavilyTool(searcher)
		if err != nil {
			return fmt.Errorf("Tavily tool initialization failed: %w", err)
		}

		tools = append(tools, searchTool)

		log.Println("Web search tool enabled: Tavily")
	} else {
		log.Println("Web search tool disabled")
	}

	// 8. Initialize AI Agent.
	mcqueenAgent, err := agent.New(
		router,
		string(prompt),
		tools,
		4,
	)
	if err != nil {
		return fmt.Errorf("AI Agent initialization failed: %w", err)
	}

	// 9. Initialize Discord bot.
	bot, err := discord.New(
		cfg.DiscordToken,
		mcqueenAgent,
		conversationManager,
		userMemory,
	)
	if err != nil {
		return fmt.Errorf("Discord initialization failed: %w", err)
	}

	if ctx.Err() != nil {
		return nil
	}

	// 10. Start Discord connection.
	if err := bot.Start(); err != nil {
		return fmt.Errorf("Discord connection failed: %w", err)
	}

	log.Printf("%s is running!", cfg.AppName)

	<-ctx.Done()

	log.Println("Shutting down...")

	if err := bot.Close(); err != nil {
		log.Printf("Discord shutdown error: %v", err)
	}

	log.Println("Application stopped")

	return nil
}

func getStringEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	return strconv.ParseBool(value)
}

func getIntEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	return strconv.Atoi(value)
}

func getDurationEnv(
	key string,
	fallback time.Duration,
) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	return time.ParseDuration(value)
}
