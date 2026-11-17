package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatWithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}

			if r.URL.Path != "/v1/chat/completions" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Error("missing authorization header")
			}

			var request chatRequest

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}

			if request.ToolChoice != "auto" {
				t.Errorf("expected tool_choice auto, got %q", request.ToolChoice)
			}

			if len(request.Tools) != 1 {
				t.Errorf("expected 1 tool, got %d", len(request.Tools))
			}

			if request.Tools[0].Function.Name != "web_search" {
				t.Errorf(
					"unexpected tool name: %s",
					request.Tools[0].Function.Name,
				)
			}

			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"choices": [
					{
						"message": {
							"role": "assistant",
							"content": null,
							"tool_calls": [
								{
									"id": "call_test_123",
									"type": "function",
									"function": {
										"name": "web_search",
										"arguments": "{\"query\":\"latest Uma Musume news\"}"
									}
								}
							]
						}
					}
				]
			}`))
		},
	))
	defer server.Close()

	router, err := NewRouter(RouterConfig{
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: time.Second * 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "web_search",
				Description: "Search the internet",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {"type": "string"}
					},
					"required": ["query"]
				}`),
			},
		},
	}

	result, err := router.ChatWithTools(
		context.Background(),
		[]Message{
			{
				Role:    "user",
				Content: "Find latest Uma Musume news",
			},
		},
		tools,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	call := result.ToolCalls[0]

	if call.Function.Name != "web_search" {
		t.Errorf("unexpected function: %s", call.Function.Name)
	}

	if !strings.Contains(call.Function.Arguments, "Uma Musume") {
		t.Errorf("unexpected arguments: %s", call.Function.Arguments)
	}
}

func TestChatWithToolsHandlesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		},
	))
	defer server.Close()

	router, err := NewRouter(RouterConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = router.ChatWithTools(
		context.Background(),
		[]Message{{Role: "user", Content: "Hello"}},
		nil,
	)

	if err == nil {
		t.Fatal("expected API error")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got: %v", err)
	}
}

func TestChatPreservesNormalChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var request chatRequest

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}

			if len(request.Tools) != 0 {
				t.Error("normal chat should not send tools")
			}

			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"choices": [
					{
						"message": {
							"role": "assistant",
							"content": "Good evening."
						}
					}
				]
			}`))
		},
	))
	defer server.Close()

	router, err := NewRouter(RouterConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := router.Chat(
		context.Background(),
		[]Message{{Role: "user", Content: "Hello"}},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result != "Good evening." {
		t.Errorf("unexpected response: %s", result)
	}
}
