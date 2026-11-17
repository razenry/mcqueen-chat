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

const maxResponseBody = 4 << 20

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
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Temperature float64          `json:"temperature"`
	MaxTokens   int              `json:"max_tokens"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  string           `json:"tool_choice,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func NewRouter(cfg RouterConfig) (*RouterClient, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" ||
		strings.TrimSpace(cfg.APIKey) == "" ||
		strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf(
			"AI base URL, API key, and model are required",
		)
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	return &RouterClient{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// Chat menjalankan percakapan biasa tanpa tools.
func (r *RouterClient) Chat(
	ctx context.Context,
	messages []Message,
) (string, error) {
	result, err := r.ChatWithTools(ctx, messages, nil)
	if err != nil {
		return "", err
	}

	content := strings.TrimSpace(result.Content)
	if content == "" {
		return "", fmt.Errorf("AI returned empty content")
	}

	return content, nil
}

// ChatWithTools mengirim pesan beserta definisi tools ke AI.
func (r *RouterClient) ChatWithTools(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
) (Message, error) {
	payload := chatRequest{
		Model:       r.config.Model,
		Messages:    messages,
		Temperature: r.config.Temperature,
		MaxTokens:   r.config.MaxTokens,
		Tools:       tools,
	}

	if len(tools) > 0 {
		payload.ToolChoice = "auto"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := r.config.BaseURL + "/chat/completions"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return Message{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("send AI request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		return Message{}, fmt.Errorf(
			"AI API returned status %d: %s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	var result chatResponse

	if err := json.NewDecoder(
		io.LimitReader(resp.Body, maxResponseBody),
	).Decode(&result); err != nil {
		return Message{}, fmt.Errorf(
			"decode AI response: %w",
			err,
		)
	}

	if len(result.Choices) == 0 {
		return Message{}, fmt.Errorf("AI returned no choices")
	}

	message := result.Choices[0].Message

	message.Content = strings.TrimSpace(message.Content)

	if message.Content == "" && len(message.ToolCalls) == 0 {
		return Message{}, fmt.Errorf(
			"AI returned neither content nor tool calls",
		)
	}

	return message, nil
}
