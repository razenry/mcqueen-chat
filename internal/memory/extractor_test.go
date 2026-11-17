package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/razenry/mcqueen-ai/internal/ai"
)

type fakeProvider struct {
	response string
	err      error
}

func (f *fakeProvider) Chat(
	ctx context.Context,
	messages []ai.Message,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	return f.response, nil
}

func TestExtractorExtractValidMemories(t *testing.T) {
	provider := &fakeProvider{
		response: `{
			"memories": [
				{
					"category": "interest",
					"content": "The user is interested in Go programming.",
					"importance": 3
				}
			]
		}`,
	}

	extractor, err := NewExtractor(provider)
	if err != nil {
		t.Fatalf("NewExtractor() error = %v", err)
	}

	memories, err := extractor.Extract(
		context.Background(),
		"I am learning Go programming.",
		"That sounds interesting!",
	)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}

	if memories[0].Category != "interest" {
		t.Errorf(
			"expected category interest, got %s",
			memories[0].Category,
		)
	}
}

func TestExtractorRejectsInvalidCategory(t *testing.T) {
	provider := &fakeProvider{
		response: `{
			"memories": [
				{
					"category": "password",
					"content": "Some information",
					"importance": 3
				}
			]
		}`,
	}

	extractor, _ := NewExtractor(provider)

	memories, err := extractor.Extract(
		context.Background(),
		"Hello",
		"Hello!",
	)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(memories) != 0 {
		t.Errorf("expected invalid category to be rejected")
	}
}

func TestExtractorRejectsSensitiveContent(t *testing.T) {
	provider := &fakeProvider{
		response: `{
			"memories": [
				{
					"category": "project",
					"content": "The user's API key is secret.",
					"importance": 5
				}
			]
		}`,
	}

	extractor, _ := NewExtractor(provider)

	memories, err := extractor.Extract(
		context.Background(),
		"Here is my API key.",
		"Thank you.",
	)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(memories) != 0 {
		t.Errorf("expected sensitive content to be rejected")
	}
}

func TestExtractorRejectsInvalidJSON(t *testing.T) {
	provider := &fakeProvider{
		response: `{"memories": [}`,
	}

	extractor, _ := NewExtractor(provider)

	_, err := extractor.Extract(
		context.Background(),
		"Hello",
		"Hello!",
	)

	if err == nil {
		t.Fatal("expected JSON decoding error")
	}
}

func TestExtractorReturnsProviderError(t *testing.T) {
	provider := &fakeProvider{
		err: errors.New("provider unavailable"),
	}

	extractor, _ := NewExtractor(provider)

	_, err := extractor.Extract(
		context.Background(),
		"Hello",
		"Hello!",
	)

	if err == nil {
		t.Fatal("expected provider error")
	}
}
