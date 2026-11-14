package websearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewTavilyClientRequiresAPIKey(t *testing.T) {
	_, err := NewTavilyClient(TavilyConfig{})

	if err == nil {
		t.Fatal("expected API key validation error")
	}
}

func TestNewTavilyClientRejectsInvalidSearchDepth(t *testing.T) {
	_, err := NewTavilyClient(TavilyConfig{
		APIKey:      "test-key",
		SearchDepth: "invalid",
	})

	if err == nil {
		t.Fatal("expected invalid search depth error")
	}
}

func TestNewTavilyClientRejectsTooManyResults(t *testing.T) {
	_, err := NewTavilyClient(TavilyConfig{
		APIKey:     "test-key",
		MaxResults: 21,
	})

	if err == nil {
		t.Fatal("expected max results validation error")
	}
}

func TestSearchRejectsEmptyQuery(t *testing.T) {
	client, err := NewTavilyClient(TavilyConfig{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Search(context.Background(), "   ")

	if err == nil {
		t.Fatal("expected empty query error")
	}
}

func TestSearchReturnsResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf(
					"expected POST, got %s",
					r.Method,
				)
			}

			if r.URL.Path != "/search" {
				t.Errorf(
					"unexpected path: %s",
					r.URL.Path,
				)
			}

			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Error("missing authorization header")
			}

			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("missing JSON content type")
			}

			var request tavilySearchRequest

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}

			if request.Query != "Uma Musume latest news" {
				t.Errorf(
					"unexpected query: %s",
					request.Query,
				)
			}

			if request.SearchDepth != "basic" {
				t.Errorf(
					"unexpected search depth: %s",
					request.SearchDepth,
				)
			}

			if request.Topic != "general" {
				t.Errorf(
					"unexpected topic: %s",
					request.Topic,
				)
			}

			if !request.IncludePublishedDate {
				t.Error("expected published date to be enabled")
			}

			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"results": [
					{
						"title": "Uma Musume News",
						"url": "https://example.com/news",
						"content": "Latest Uma Musume update",
						"score": 0.95,
						"published_date": "2026-11-14"
					}
				]
			}`))
		},
	))
	defer server.Close()

	client, err := NewTavilyClient(TavilyConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := client.Search(
		context.Background(),
		"Uma Musume latest news",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf(
			"expected 1 result, got %d",
			len(results),
		)
	}

	if results[0].Title != "Uma Musume News" {
		t.Errorf(
			"unexpected title: %s",
			results[0].Title,
		)
	}

	if results[0].URL != "https://example.com/news" {
		t.Errorf(
			"unexpected URL: %s",
			results[0].URL,
		)
	}

	if results[0].Score != 0.95 {
		t.Errorf(
			"unexpected score: %f",
			results[0].Score,
		)
	}

	if results[0].PublishedDate != "2026-11-14" {
		t.Errorf(
			"unexpected published date: %s",
			results[0].PublishedDate,
		)
	}
}

func TestSearchHandlesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(
				w,
				"Unauthorized",
				http.StatusUnauthorized,
			)
		},
	))
	defer server.Close()

	client, err := NewTavilyClient(TavilyConfig{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Search(
		context.Background(),
		"Uma Musume",
	)

	if err == nil {
		t.Fatal("expected API error")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf(
			"expected HTTP 401 error, got: %v",
			err,
		)
	}
}

func TestSearchHandlesEmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[]}`))
		},
	))
	defer server.Close()

	client, err := NewTavilyClient(TavilyConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := client.Search(
		context.Background(),
		"Unknown query",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Fatalf(
			"expected empty results, got %d",
			len(results),
		)
	}
}

func TestSearchHandlesInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`not-json`))
		},
	))
	defer server.Close()

	client, err := NewTavilyClient(TavilyConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Search(
		context.Background(),
		"Uma Musume",
	)

	if err == nil {
		t.Fatal("expected JSON decoding error")
	}
}
