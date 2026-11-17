package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/razenry/mcqueen-ai/internal/ai"
)

type Agent struct {
	provider      ai.ToolProvider
	systemPrompt  string
	tools         map[string]Tool
	maxIterations int
}

func New(
	provider ai.ToolProvider,
	systemPrompt string,
	registeredTools []Tool,
	maxIterations int,
) (*Agent, error) {
	if provider == nil {
		return nil, fmt.Errorf("AI provider is required")
	}

	if maxIterations <= 0 {
		maxIterations = 4
	}

	tools := make(map[string]Tool, len(registeredTools))

	for _, tool := range registeredTools {
		if tool == nil {
			return nil, fmt.Errorf("registered tool cannot be nil")
		}

		name := strings.TrimSpace(tool.Name())

		if name == "" {
			return nil, fmt.Errorf("tool name cannot be empty")
		}

		if _, exists := tools[name]; exists {
			return nil, fmt.Errorf("duplicate tool name: %s", name)
		}

		if !json.Valid(tool.Parameters()) {
			return nil, fmt.Errorf(
				"tool %s has invalid JSON parameters",
				name,
			)
		}

		tools[name] = tool
	}

	return &Agent{
		provider:      provider,
		systemPrompt:  systemPrompt,
		tools:         tools,
		maxIterations: maxIterations,
	}, nil
}

// Run menjalankan satu siklus percakapan Agent.
func (a *Agent) Run(
	ctx context.Context,
	messages []ai.Message,
) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("conversation messages are required")
	}

	conversation := make([]ai.Message, 0, len(messages)+8)

	if strings.TrimSpace(a.systemPrompt) != "" {
		conversation = append(conversation, ai.Message{
			Role:    "system",
			Content: a.systemPrompt,
		})
	}

	conversation = append(conversation, messages...)

	definitions := a.toolDefinitions()

	// Menyimpan fingerprint pemanggilan tool dalam satu siklus.
	// Ini mencegah query yang sama dieksekusi berulang kali.
	executedCalls := make(map[string]struct{})

	for iteration := 1; iteration <= a.maxIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		log.Printf(
			"[Agent] Iteration %d/%d",
			iteration,
			a.maxIterations,
		)

		response, err := a.provider.ChatWithTools(
			ctx,
			conversation,
			definitions,
		)
		if err != nil {
			return "", fmt.Errorf("agent AI request: %w", err)
		}

		// Tidak ada tool call berarti AI sudah menghasilkan jawaban akhir.
		if len(response.ToolCalls) == 0 {
			if strings.TrimSpace(response.Content) == "" {
				return "", fmt.Errorf("agent received empty final response")
			}

			log.Printf("[Agent] Final answer generated")

			return response.Content, nil
		}

		response.Role = "assistant"
		conversation = append(conversation, response)

		for _, call := range response.ToolCalls {
			if err := ctx.Err(); err != nil {
				return "", err
			}

			if call.ID == "" {
				return "", fmt.Errorf("AI returned tool call without ID")
			}

			log.Printf(
				"[Agent] Tool requested: %s",
				call.Function.Name,
			)

			fingerprint, err := toolFingerprint(call)
			if err != nil {
				return "", fmt.Errorf(
					"create tool fingerprint: %w",
					err,
				)
			}

			var result string

			if _, exists := executedCalls[fingerprint]; exists {
				log.Printf(
					"[Agent] Duplicate tool call blocked: %s",
					call.Function.Name,
				)

				result = toolError(
					"This exact tool request was already executed. " +
						"Use the existing search results to answer the user. " +
						"Do not repeat the same search.",
				)
			} else {
				executedCalls[fingerprint] = struct{}{}

				result = a.executeTool(ctx, call)
			}

			conversation = append(conversation, ai.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}

	// Batas putaran tool tercapai.
	// Sekarang minta AI menyusun jawaban akhir tanpa memberikan tools.
	log.Printf(
		"[Agent] Tool iteration limit reached. Requesting final answer without tools.",
	)

	if err := ctx.Err(); err != nil {
		return "", err
	}

	finalResponse, err := a.provider.ChatWithTools(
		ctx,
		conversation,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf(
			"agent finalization request: %w",
			err,
		)
	}

	if len(finalResponse.ToolCalls) > 0 {
		return "", fmt.Errorf(
			"AI requested tools during finalization",
		)
	}

	finalAnswer := strings.TrimSpace(finalResponse.Content)

	if finalAnswer == "" {
		return "", fmt.Errorf(
			"agent received empty response during finalization",
		)
	}

	log.Printf("[Agent] Final answer generated after tool limit")

	return finalAnswer, nil
}

// toolDefinitions mengubah tools menjadi format OpenAI-compatible.
func (a *Agent) toolDefinitions() []ai.ToolDefinition {
	definitions := make([]ai.ToolDefinition, 0, len(a.tools))

	for _, tool := range a.tools {
		definitions = append(definitions, ai.ToolDefinition{
			Type: "function",
			Function: ai.FunctionDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}

	return definitions
}

// executeTool menjalankan tool yang diminta AI.
func (a *Agent) executeTool(
	ctx context.Context,
	call ai.ToolCall,
) string {
	if call.Type != "" && call.Type != "function" {
		return toolError("unsupported tool call type")
	}

	tool, exists := a.tools[call.Function.Name]
	if !exists {
		return toolError(
			fmt.Sprintf("unknown tool: %s", call.Function.Name),
		)
	}

	arguments := json.RawMessage(call.Function.Arguments)

	if !json.Valid(arguments) {
		return toolError("invalid tool arguments JSON")
	}

	result, err := tool.Execute(ctx, arguments)
	if err != nil {
		log.Printf(
			"[Agent] Tool execution failed: %s: %v",
			call.Function.Name,
			err,
		)

		return toolError(err.Error())
	}

	log.Printf(
		"[Agent] Tool execution successful: %s",
		call.Function.Name,
	)

	return result
}

// toolFingerprint menghasilkan identitas unik dari nama tool dan argumennya.

func toolFingerprint(call ai.ToolCall) (string, error) {
	arguments := strings.TrimSpace(call.Function.Arguments)

	var normalized any

	// Normalize valid JSON so equivalent arguments
	// produce the same fingerprint.
	if err := json.Unmarshal(
		[]byte(arguments),
		&normalized,
	); err == nil {
		if encoded, err := json.Marshal(normalized); err == nil {
			arguments = string(encoded)
		}
	}

	// Invalid JSON is still fingerprinted using
	// its original raw arguments.
	payload := call.Function.Name + ":" + arguments

	hash := sha256.Sum256([]byte(payload))

	return fmt.Sprintf("%x", hash), nil
}

func toolError(message string) string {
	result, _ := json.Marshal(map[string]string{
		"error": message,
	})

	return string(result)
}
