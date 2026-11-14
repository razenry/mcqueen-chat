package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/razenry/mcqueen-ai/internal/tools/websearch"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	// Load environment variables from .env.
	// Existing environment variables take precedence.
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found; using system environment")
	}

	if len(os.Args) < 2 {
		return fmt.Errorf(
			"usage: go run ./cmd/searchtest \"your search query\"",
		)
	}

	query := strings.TrimSpace(
		strings.Join(os.Args[1:], " "),
	)

	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("TAVILY_API_KEY is not configured")
	}

	maxResults := 5

	if value := os.Getenv("TAVILY_MAX_RESULTS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid TAVILY_MAX_RESULTS: %w", err)
		}
		maxResults = parsed
	}

	timeout := 15 * time.Second

	if value := os.Getenv("TAVILY_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid TAVILY_TIMEOUT: %w", err)
		}
		timeout = parsed
	}

	client, err := websearch.NewTavilyClient(
		websearch.TavilyConfig{
			APIKey:     apiKey,
			BaseURL:    os.Getenv("TAVILY_BASE_URL"),
			MaxResults: maxResults,
			SearchDepth: envOrDefault(
				"TAVILY_SEARCH_DEPTH",
				"basic",
			),
			Topic: envOrDefault(
				"TAVILY_TOPIC",
				"general",
			),
			Timeout: timeout,
		},
	)
	if err != nil {
		return fmt.Errorf("initialize Tavily client: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancel()

	fmt.Println()
	fmt.Println("McQueen Web Search")
	fmt.Println("==================")
	fmt.Println("Query:", query)
	fmt.Println("Searching...")
	fmt.Println()

	results, err := client.Search(ctx, query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No search results found.")
		return nil
	}

	fmt.Printf("Found %d results\n\n", len(results))

	for i, result := range results {
		fmt.Printf("[%d] %s\n", i+1, result.Title)
		fmt.Printf("URL: %s\n", result.URL)

		if result.PublishedDate != "" {
			fmt.Printf("Published: %s\n", result.PublishedDate)
		}

		fmt.Printf("Relevance: %.2f\n", result.Score)
		fmt.Printf("Content: %s\n", result.Content)
		fmt.Println(strings.Repeat("-", 60))
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
