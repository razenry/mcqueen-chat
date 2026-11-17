
-- +goose Up

-- Satu user hanya boleh memiliki satu percakapan aktif
-- pada channel Discord yang sama.
CREATE UNIQUE INDEX idx_conversations_one_active
ON conversations (discord_user_id, channel_id)
WHERE status = 'active';


-- +goose Down

DROP INDEX IF EXISTS idx_conversations_one_active;
