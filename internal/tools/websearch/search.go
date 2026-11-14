package websearch

import "context"

// Result represents one search result returned by a provider.
type Result struct {
	Title         string
	URL           string
	Content       string
	Score         float64
	PublishedDate string
}

// Searcher defines the contract for web search providers.
//
// Any provider that implements Search can be used by
// the AI Agent without depending on a specific service.
type Searcher interface {
	Search(ctx context.Context, query string) ([]Result, error)
}
