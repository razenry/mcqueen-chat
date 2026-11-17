package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/razenry/mcqueen-ai/internal/agent"
	"github.com/razenry/mcqueen-ai/internal/ai"
	"github.com/razenry/mcqueen-ai/internal/tools/websearch"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Agent error:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		return fmt.Errorf(
			"usage: go run ./cmd/agenttest \"your message\"",
		)
	}

	apiKey := os.Getenv("AI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")

	if apiKey == "" || baseURL == "" || model == "" {
		return fmt.Errorf(
			"AI_BASE_URL, AI_API_KEY, and AI_MODEL are required",
		)
	}

	tavilyKey := os.Getenv("TAVILY_API_KEY")
	if tavilyKey == "" {
		return fmt.Errorf("TAVILY_API_KEY is required")
	}

	timeout := 60 * time.Second

	if value := os.Getenv("AI_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("invalid AI_TIMEOUT")
		}
		timeout = parsed
	}

	temperature := 0.8

	if value := os.Getenv("AI_TEMPERATURE"); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || parsed < 0 || parsed > 2 {
			return fmt.Errorf("invalid AI_TEMPERATURE")
		}
		temperature = parsed
	}

	maxTokens := 1000

	if value := os.Getenv("AI_MAX_TOKENS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("invalid AI_MAX_TOKENS")
		}
		maxTokens = parsed
	}

	router, err := ai.NewRouter(ai.RouterConfig{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Timeout:     timeout,
	})
	if err != nil {
		return err
	}

	searchTimeout := 15 * time.Second

	if value := os.Getenv("TAVILY_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("invalid TAVILY_TIMEOUT")
		}
		searchTimeout = parsed
	}

	maxResults := 5

	if value := os.Getenv("TAVILY_MAX_RESULTS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 20 {
			return fmt.Errorf("invalid TAVILY_MAX_RESULTS")
		}
		maxResults = parsed
	}

	searcher, err := websearch.NewTavilyClient(
		websearch.TavilyConfig{
			APIKey:      tavilyKey,
			BaseURL:     envOrDefault("TAVILY_BASE_URL", "https://api.tavily.com"),
			MaxResults:  maxResults,
			SearchDepth: envOrDefault("TAVILY_SEARCH_DEPTH", "basic"),
			Topic:       envOrDefault("TAVILY_TOPIC", "general"),
			Timeout:     searchTimeout,
		},
	)
	if err != nil {
		return err
	}

	searchTool, err := websearch.NewTavilyTool(searcher)
	if err != nil {
		return err
	}

	systemPrompt := loadSystemPrompt()

	mcqueen, err := agent.New(
		router,
		systemPrompt,
		[]agent.Tool{
			searchTool,
		},
		4,
	)
	if err != nil {
		return err
	}

	userMessage := strings.Join(os.Args[1:], " ")

	messages := []ai.Message{
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancel()

	fmt.Println()
	fmt.Println("McQueen AI Agent")
	fmt.Println("================")
	fmt.Println("Model:", model)
	fmt.Println("Message:", userMessage)
	fmt.Println()
	fmt.Println("Agent is thinking...")
	fmt.Println()

	response, err := mcqueen.Run(ctx, messages)
	if err != nil {
		return err
	}

	fmt.Println("McQueen:")
	fmt.Println(response)

	return nil
}

func loadSystemPrompt() string {
	content, err := os.ReadFile("prompts/mcqueen.txt")
	if err != nil {
		return "You are Mejiro McQueen, an elegant and polite Uma Musume-inspired assistant. Respond naturally in Indonesian. Keep answers concise unless detail is necessary. Use web_search when current or changing information is needed. Never invent search results or sources."
	}

	return strings.TrimSpace(string(content))
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback
	}

	return value
}
