package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/razenry/mcqueen-ai/internal/ai"
)

type providerCall struct {
	messages []ai.Message
	tools    []ai.ToolDefinition
}

type fakeProvider struct {
	responses []ai.Message
	calls     int
	requests  []providerCall
}

func (f *fakeProvider) ChatWithTools(
	ctx context.Context,
	messages []ai.Message,
	tools []ai.ToolDefinition,
) (ai.Message, error) {
	if err := ctx.Err(); err != nil {
		return ai.Message{}, err
	}

	messageCopy := append([]ai.Message(nil), messages...)
	toolCopy := append([]ai.ToolDefinition(nil), tools...)

	f.requests = append(f.requests, providerCall{
		messages: messageCopy,
		tools:    toolCopy,
	})

	if f.calls >= len(f.responses) {
		return ai.Message{}, context.DeadlineExceeded
	}

	response := f.responses[f.calls]
	f.calls++

	return response, nil
}

type fakeTool struct {
	executions int
	lastQuery  string
}

func (f *fakeTool) Name() string {
	return "web_search"
}

func (f *fakeTool) Description() string {
	return "Search the internet"
}

func (f *fakeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string"
			}
		},
		"required": ["query"],
		"additionalProperties": false
	}`)
}

func (f *fakeTool) Execute(
	ctx context.Context,
	args json.RawMessage,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var input struct {
		Query string `json:"query"`
	}

	if err := json.Unmarshal(args, &input); err != nil {
		return "", err
	}

	if input.Query == "" {
		return "", context.Canceled
	}

	f.executions++
	f.lastQuery = input.Query

	return `{"results":[{"title":"Test News"}]}`, nil
}

func searchToolCall(id, query string) ai.Message {
	return ai.Message{
		Role: "assistant",
		ToolCalls: []ai.ToolCall{
			{
				ID:   id,
				Type: "function",
				Function: ai.ToolCallFunction{
					Name:      "web_search",
					Arguments: `{"query":"` + query + `"}`,
				},
			},
		},
	}
}

func userMessage(content string) []ai.Message {
	return []ai.Message{
		{
			Role:    "user",
			Content: content,
		},
	}
}

func TestAgentExecutesToolAndReturnsFinalAnswer(t *testing.T) {
	provider := &fakeProvider{
		responses: []ai.Message{
			searchToolCall("call_123", "latest Uma Musume news"),
			{
				Role:    "assistant",
				Content: "Here are the latest Uma Musume news.",
			},
		},
	}

	tool := &fakeTool{}

	instance, err := New(
		provider,
		"You are McQueen.",
		[]Tool{tool},
		4,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := instance.Run(
		context.Background(),
		userMessage("Find the latest news"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if tool.executions != 1 {
		t.Fatalf(
			"expected tool to execute once, got %d",
			tool.executions,
		)
	}

	if provider.calls != 2 {
		t.Fatalf(
			"expected 2 AI calls, got %d",
			provider.calls,
		)
	}

	if !strings.Contains(result, "latest Uma Musume news") {
		t.Errorf("unexpected final response: %s", result)
	}

	if len(provider.requests) < 2 {
		t.Fatal("expected at least 2 provider requests")
	}

	foundToolResult := false

	for _, message := range provider.requests[1].messages {
		if message.Role == "tool" &&
			message.ToolCallID == "call_123" {
			foundToolResult = true
			break
		}
	}

	if !foundToolResult {
		t.Fatal("expected tool result in second AI request")
	}

	if len(provider.requests[0].tools) != 1 {
		t.Fatal("expected web_search definition in first request")
	}
}

func TestAgentReturnsDirectAnswerWithoutTool(t *testing.T) {
	provider := &fakeProvider{
		responses: []ai.Message{
			{
				Role:    "assistant",
				Content: "Good evening, Trainer.",
			},
		},
	}

	instance, err := New(
		provider,
		"You are McQueen.",
		nil,
		4,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := instance.Run(
		context.Background(),
		userMessage("Hello"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if result != "Good evening, Trainer." {
		t.Errorf("unexpected response: %s", result)
	}

	if provider.calls != 1 {
		t.Errorf(
			"expected 1 AI call, got %d",
			provider.calls,
		)
	}
}

func TestAgentPreventsDuplicateToolExecution(t *testing.T) {
	provider := &fakeProvider{
		responses: []ai.Message{
			searchToolCall("call_1", "latest Uma Musume news"),
			searchToolCall("call_2", "latest Uma Musume news"),
			{
				Role:    "assistant",
				Content: "Here is the search summary.",
			},
		},
	}

	tool := &fakeTool{}

	instance, err := New(
		provider,
		"You are McQueen.",
		[]Tool{tool},
		2,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := instance.Run(
		context.Background(),
		userMessage("Find the latest news"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if tool.executions != 1 {
		t.Fatalf(
			"expected duplicate execution to be blocked; got %d executions",
			tool.executions,
		)
	}

	if provider.calls != 3 {
		t.Fatalf(
			"expected 3 AI calls including finalization, got %d",
			provider.calls,
		)
	}

	if !strings.Contains(result, "search summary") {
		t.Errorf("unexpected final response: %s", result)
	}

	// The third request is finalization.
	if len(provider.requests[2].tools) != 0 {
		t.Fatal("expected finalization request without tools")
	}

	// The duplicate result should be returned to the AI.
	foundDuplicateNotice := false

	for _, message := range provider.requests[2].messages {
		if message.Role == "tool" &&
			message.ToolCallID == "call_2" &&
			strings.Contains(message.Content, "already executed") {
			foundDuplicateNotice = true
			break
		}
	}

	if !foundDuplicateNotice {
		t.Fatal("expected duplicate warning in finalization context")
	}
}

func TestAgentFinalizesAfterMaximumIterations(t *testing.T) {
	provider := &fakeProvider{
		responses: []ai.Message{
			searchToolCall("call_1", "Uma Musume news"),
			searchToolCall("call_2", "Uma Musume official announcements"),
			{
				Role:    "assistant",
				Content: "Here is the final summary.",
			},
		},
	}

	tool := &fakeTool{}

	instance, err := New(
		provider,
		"You are McQueen.",
		[]Tool{tool},
		2,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := instance.Run(
		context.Background(),
		userMessage("Find the latest news"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if tool.executions != 2 {
		t.Fatalf(
			"expected 2 tool executions, got %d",
			tool.executions,
		)
	}

	if provider.calls != 3 {
		t.Fatalf(
			"expected 3 AI calls including finalization, got %d",
			provider.calls,
		)
	}

	if result != "Here is the final summary." {
		t.Errorf("unexpected final response: %s", result)
	}

	lastRequest := provider.requests[len(provider.requests)-1]

	if len(lastRequest.tools) != 0 {
		t.Fatal("expected finalization request without tools")
	}

	foundToolResults := 0

	for _, message := range lastRequest.messages {
		if message.Role == "tool" {
			foundToolResults++
		}
	}

	if foundToolResults != 2 {
		t.Errorf(
			"expected 2 tool results in finalization context, got %d",
			foundToolResults,
		)
	}
}

func TestAgentRejectsDuplicateTools(t *testing.T) {
	provider := &fakeProvider{}

	_, err := New(
		provider,
		"",
		[]Tool{
			&fakeTool{},
			&fakeTool{},
		},
		4,
	)

	if err == nil {
		t.Fatal("expected duplicate tool error")
	}
}

func TestAgentRejectsInvalidToolArguments(t *testing.T) {
	provider := &fakeProvider{
		responses: []ai.Message{
			{
				Role: "assistant",
				ToolCalls: []ai.ToolCall{
					{
						ID:   "call_invalid",
						Type: "function",
						Function: ai.ToolCallFunction{
							Name:      "web_search",
							Arguments: `{invalid json}`,
						},
					},
				},
			},
			{
				Role:    "assistant",
				Content: "I could not process that search.",
			},
		},
	}

	tool := &fakeTool{}

	instance, err := New(
		provider,
		"",
		[]Tool{tool},
		4,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = instance.Run(
		context.Background(),
		userMessage("Search something"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if tool.executions != 0 {
		t.Fatal("tool should not execute with invalid arguments")
	}

	foundError := false

	for _, message := range provider.requests[1].messages {
		if message.Role == "tool" &&
			message.ToolCallID == "call_invalid" &&
			strings.Contains(message.Content, "invalid tool arguments") {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Fatal("expected tool error to be returned to AI")
	}
}
