
-- +goose Up

CREATE INDEX idx_memories_search
ON memories
USING GIN (
    to_tsvector('simple', category || ' ' || content)
)
WHERE is_active = TRUE;

-- +goose Down

DROP INDEX IF EXISTS idx_memories_search;
