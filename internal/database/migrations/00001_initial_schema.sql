
-- +goose Up

CREATE TABLE users (
    discord_user_id TEXT PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    global_name VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE conversations (
    id BIGSERIAL PRIMARY KEY,
    discord_user_id TEXT NOT NULL
        REFERENCES users(discord_user_id)
        ON DELETE CASCADE,

    channel_id TEXT NOT NULL,
    guild_id TEXT,

    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_conversations_user_channel
    ON conversations (
        discord_user_id,
        channel_id,
        last_message_at DESC
    );

CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,

    conversation_id BIGINT NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    role VARCHAR(20) NOT NULL
        CHECK (role IN ('user', 'assistant', 'system', 'tool')),

    content TEXT NOT NULL,
    tool_call_id TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_conversation_created
    ON messages (conversation_id, created_at, id);

CREATE TABLE memories (
    id BIGSERIAL PRIMARY KEY,

    discord_user_id TEXT NOT NULL
        REFERENCES users(discord_user_id)
        ON DELETE CASCADE,

    category VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,

    source_conversation_id BIGINT
        REFERENCES conversations(id)
        ON DELETE SET NULL,

    importance SMALLINT NOT NULL DEFAULT 3
        CHECK (importance BETWEEN 1 AND 5),

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ
);

CREATE INDEX idx_memories_user_active
    ON memories (discord_user_id, is_active);

-- +goose Down

DROP TABLE IF EXISTS memories;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS users;
