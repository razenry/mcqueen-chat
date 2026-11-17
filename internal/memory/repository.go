package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Memory struct {
	ID         int64
	Category   string
	Content    string
	Importance int16
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Candidate struct {
	Category   string `json:"category"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("database pool is required")
	}

	return &Repository{db: db}, nil
}

func (r *Repository) ListActive(
	ctx context.Context,
	userID string,
	limit int,
) ([]Memory, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			category,
			content,
			importance,
			created_at,
			updated_at
		FROM memories
		WHERE discord_user_id = $1
		  AND is_active = TRUE
		ORDER BY importance DESC, updated_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	return scanMemories(rows, limit)
}

// SearchRelevant mencari memori berdasarkan kemiripan kata dengan query.
func (r *Repository) SearchRelevant(
	ctx context.Context,
	userID string,
	query string,
	limit int,
) ([]Memory, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return r.ListActive(ctx, userID, limit)
	}

	// Batasi query agar pencarian tidak menerima teks yang terlalu panjang.
	runes := []rune(query)
	if len(runes) > 500 {
		query = string(runes[:500])
	}

	if limit <= 0 || limit > 20 {
		limit = 5
	}

	rows, err := r.db.Query(ctx, `
		WITH search_query AS (
			SELECT websearch_to_tsquery('simple', $2) AS terms
		),
		matched_memories AS (
			SELECT
				m.id,
				m.category,
				m.content,
				m.importance,
				m.created_at,
				m.updated_at,
				ts_rank_cd(
					to_tsvector(
						'simple',
						m.category || ' ' || m.content
					),
					q.terms
				) AS relevance
			FROM memories AS m
			CROSS JOIN search_query AS q
			WHERE m.discord_user_id = $1
			  AND m.is_active = TRUE
			  AND q.terms <> ''::tsquery
			  AND to_tsvector(
					'simple',
					m.category || ' ' || m.content
			  ) @@ q.terms
		)
		SELECT
			id,
			category,
			content,
			importance,
			created_at,
			updated_at
		FROM matched_memories
		ORDER BY relevance DESC, importance DESC, updated_at DESC
		LIMIT $3
	`, userID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search relevant memories: %w", err)
	}
	defer rows.Close()

	return scanMemories(rows, limit)
}

func scanMemories(
	rows pgx.Rows,
	capacity int,
) ([]Memory, error) {
	memories := make([]Memory, 0, capacity)

	for rows.Next() {
		var item Memory

		if err := rows.Scan(
			&item.ID,
			&item.Category,
			&item.Content,
			&item.Importance,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}

		memories = append(memories, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memories: %w", err)
	}

	return memories, nil
}

func (r *Repository) Save(
	ctx context.Context,
	userID string,
	candidate Candidate,
) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user ID is required")
	}

	candidate.Category = strings.TrimSpace(candidate.Category)
	candidate.Content = strings.TrimSpace(candidate.Content)

	if candidate.Category == "" || candidate.Content == "" {
		return fmt.Errorf("memory category and content are required")
	}

	if candidate.Importance < 1 || candidate.Importance > 5 {
		return fmt.Errorf("memory importance must be between 1 and 5")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin memory transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var existingID int64

	err = tx.QueryRow(ctx, `
		SELECT id
		FROM memories
		WHERE discord_user_id = $1
		  AND is_active = TRUE
		  AND LOWER(TRIM(content)) = LOWER(TRIM($2))
		LIMIT 1
	`, userID, candidate.Content).Scan(&existingID)

	switch {
	case err == nil:
		_, err = tx.Exec(ctx, `
			UPDATE memories
			SET
				category = $3,
				importance = GREATEST(importance, $4),
				updated_at = NOW()
			WHERE id = $1
			  AND discord_user_id = $2
		`, existingID, userID, candidate.Category, candidate.Importance)

		if err != nil {
			return fmt.Errorf("update existing memory: %w", err)
		}

	case err == pgx.ErrNoRows:
		_, err = tx.Exec(ctx, `
			INSERT INTO memories (
				discord_user_id,
				category,
				content,
				importance
			)
			VALUES ($1, $2, $3, $4)
		`, userID, candidate.Category, candidate.Content, candidate.Importance)

		if err != nil {
			return fmt.Errorf("insert memory: %w", err)
		}

	default:
		return fmt.Errorf("find existing memory: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit memory transaction: %w", err)
	}

	return nil
}

func (r *Repository) Deactivate(
	ctx context.Context,
	userID string,
	memoryID int64,
) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user ID is required")
	}

	if memoryID <= 0 {
		return fmt.Errorf("invalid memory ID")
	}

	commandTag, err := r.db.Exec(ctx, `
		UPDATE memories
		SET
			is_active = FALSE,
			updated_at = NOW()
		WHERE id = $1
		  AND discord_user_id = $2
		  AND is_active = TRUE
	`, memoryID, userID)
	if err != nil {
		return fmt.Errorf("deactivate memory: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("memory not found")
	}

	return nil
}
