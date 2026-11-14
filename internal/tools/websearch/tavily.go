package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://api.tavily.com"
	defaultMaxResults = 5
	defaultDepth      = "basic"
	defaultTopic      = "general"
	maxResponseSize   = 2 << 20 // 2 MiB
	maxQueryLength    = 400
)

// TavilyConfig contains the configuration required by TavilyClient.
type TavilyConfig struct {
	APIKey      string
	BaseURL     string
	MaxResults  int
	SearchDepth string
	Topic       string
	Timeout     time.Duration
}

// TavilyClient communicates with the Tavily Search API.
type TavilyClient struct {
	config     TavilyConfig
	httpClient *http.Client
}

// tavilySearchRequest represents the JSON request sent to Tavily.
type tavilySearchRequest struct {
	Query                string `json:"query"`
	SearchDepth          string `json:"search_depth"`
	Topic                string `json:"topic"`
	MaxResults           int    `json:"max_results"`
	IncludeAnswer        bool   `json:"include_answer"`
	IncludeRawContent    bool   `json:"include_raw_content"`
	IncludePublishedDate bool   `json:"include_published_date"`
}

// tavilySearchResponse represents the relevant parts of Tavily's response.
type tavilySearchResponse struct {
	Results []struct {
		Title         string  `json:"title"`
		URL           string  `json:"url"`
		Content       string  `json:"content"`
		Score         float64 `json:"score"`
		PublishedDate string  `json:"published_date"`
	} `json:"results"`
}

// NewTavilyClient creates and validates a Tavily client.
func NewTavilyClient(cfg TavilyConfig) (*TavilyClient, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("tavily API key is required")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	parsedURL, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Tavily base URL: %w", err)
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return nil, fmt.Errorf("Tavily base URL must use HTTP or HTTPS")
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("Tavily base URL must contain a host")
	}

	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	if cfg.MaxResults <= 0 {
		cfg.MaxResults = defaultMaxResults
	}

	if cfg.MaxResults > 20 {
		return nil, fmt.Errorf("Tavily max results cannot exceed 20")
	}

	if cfg.SearchDepth == "" {
		cfg.SearchDepth = defaultDepth
	}

	switch cfg.SearchDepth {
	case "basic", "advanced", "fast", "ultra-fast":
		// Valid search depth.
	default:
		return nil, fmt.Errorf(
			"unsupported Tavily search depth: %s",
			cfg.SearchDepth,
		)
	}

	if cfg.Topic == "" {
		cfg.Topic = defaultTopic
	}

	switch cfg.Topic {
	case "general", "news", "finance":
		// Valid search topic.
	default:
		return nil, fmt.Errorf(
			"unsupported Tavily topic: %s",
			cfg.Topic,
		)
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}

	return &TavilyClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// Search executes a query against the Tavily Search API.
func (c *TavilyClient) Search(
	ctx context.Context,
	query string,
) ([]Result, error) {
	query = strings.TrimSpace(query)

	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	if len(query) > maxQueryLength {
		return nil, fmt.Errorf(
			"search query cannot exceed %d bytes",
			maxQueryLength,
		)
	}

	payload := tavilySearchRequest{
		Query:                query,
		SearchDepth:          c.config.SearchDepth,
		Topic:                c.config.Topic,
		MaxResults:           c.config.MaxResults,
		IncludeAnswer:        false,
		IncludeRawContent:    false,
		IncludePublishedDate: true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Tavily request: %w", err)
	}

	endpoint := c.config.BaseURL + "/search"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create Tavily request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute Tavily search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, readErr := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)
		if readErr != nil {
			return nil, fmt.Errorf(
				"Tavily returned HTTP %d",
				resp.StatusCode,
			)
		}

		return nil, fmt.Errorf(
			"Tavily returned HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(errorBody)),
		)
	}

	var response tavilySearchResponse

	decoder := json.NewDecoder(
		io.LimitReader(resp.Body, maxResponseSize),
	)

	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode Tavily response: %w", err)
	}

	results := make([]Result, 0, len(response.Results))

	for _, item := range response.Results {
		if strings.TrimSpace(item.Title) == "" ||
			strings.TrimSpace(item.URL) == "" {
			continue
		}

		results = append(results, Result{
			Title:         item.Title,
			URL:           item.URL,
			Content:       item.Content,
			Score:         item.Score,
			PublishedDate: item.PublishedDate,
		})
	}

	return results, nil
}
