package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RouterConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     time.Duration
}

type RouterClient struct {
	config RouterConfig
	client *http.Client
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func NewRouter(cfg RouterConfig) (*RouterClient, error) {
	if cfg.BaseURL == "" ||
		cfg.APIKey == "" ||
		cfg.Model == "" {
		return nil, fmt.Errorf(
			"AI base URL, API key, and model are required",
		)
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &RouterClient{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (r *RouterClient) Chat(
	ctx context.Context,
	messages []Message,
) (string, error) {
	payload := chatRequest{
		Model:       r.config.Model,
		Messages:    messages,
		Temperature: r.config.Temperature,
		MaxTokens:   r.config.MaxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimRight(
		r.config.BaseURL,
		"/",
	) + "/chat/completions"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send AI request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		return "", fmt.Errorf(
			"AI API returned status %d: %s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	var result chatResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode AI response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("AI returned no choices")
	}

	content := strings.TrimSpace(
		result.Choices[0].Message.Content,
	)

	if content == "" {
		return "", fmt.Errorf("AI returned empty content")
	}

	return content, nil
}
