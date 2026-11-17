package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/razenry/mcqueen-ai/internal/agent"
)

type TavilyTool struct {
	searcher Searcher
}

type searchArguments struct {
	Query string `json:"query"`
}

func NewTavilyTool(searcher Searcher) (*TavilyTool, error) {
	if searcher == nil {
		return nil, fmt.Errorf("web searcher is required")
	}

	return &TavilyTool{
		searcher: searcher,
	}, nil
}

func (t *TavilyTool) Name() string {
	return "web_search"
}

func (t *TavilyTool) Description() string {
	return "Search the internet for recent or specific information. Use this when the user asks about current news, recent updates, events, announcements, or facts that may have changed."
}

func (t *TavilyTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "A specific search query to find relevant information on the internet."
			}
		},
		"required": ["query"],
		"additionalProperties": false
	}`)
}

func (t *TavilyTool) Execute(
	ctx context.Context,
	raw json.RawMessage,
) (string, error) {
	var args searchArguments

	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("decode search arguments: %w", err)
	}

	args.Query = strings.TrimSpace(args.Query)

	if args.Query == "" {
		return "", fmt.Errorf("search query cannot be empty")
	}

	results, err := t.searcher.Search(ctx, args.Query)
	if err != nil {
		return "", fmt.Errorf("web search failed: %w", err)
	}

	if len(results) == 0 {
		return `{"results":[],"message":"No search results found."}`, nil
	}

	// Batasi hasil yang diberikan kembali kepada model.
	if len(results) > 5 {
		results = results[:5]
	}

	type searchResult struct {
		Title         string  `json:"title"`
		URL           string  `json:"url"`
		Content       string  `json:"content"`
		Score         float64 `json:"score"`
		PublishedDate string  `json:"published_date,omitempty"`
	}

	output := make([]searchResult, 0, len(results))

	for _, result := range results {
		output = append(output, searchResult{
			Title:         result.Title,
			URL:           result.URL,
			Content:       result.Content,
			Score:         result.Score,
			PublishedDate: result.PublishedDate,
		})
	}

	encoded, err := json.Marshal(map[string]any{
		"query":   args.Query,
		"results": output,
	})
	if err != nil {
		return "", fmt.Errorf("encode search results: %w", err)
	}

	return string(encoded), nil
}

// Compile-time check: TavilyTool harus memenuhi kontrak agent.Tool.
var _ agent.Tool = (*TavilyTool)(nil)
