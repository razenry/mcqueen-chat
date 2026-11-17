package conversation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/razenry/mcqueen-ai/internal/ai"
)

type SessionKey struct {
	UserID     string
	Username   string
	GlobalName string
	ChannelID  string
	GuildID    string
}

type Manager struct {
	db       *pgxpool.Pool
	maxTurns int
}

func NewManager(db *pgxpool.Pool, maxTurns int) *Manager {
	if maxTurns <= 0 {
		maxTurns = 6
	}

	return &Manager{
		db:       db,
		maxTurns: maxTurns,
	}
}

// Get mengambil maksimal maxTurns percakapan terakhir
// dari satu user pada satu channel.
func (m *Manager) Get(
	ctx context.Context,
	key SessionKey,
) ([]ai.Message, error) {
	var conversationID int64

	err := m.db.QueryRow(
		ctx,
		`
		SELECT id
		FROM conversations
		WHERE discord_user_id = $1
		  AND channel_id = $2
		  AND status = 'active'
		ORDER BY last_message_at DESC
		LIMIT 1
		`,
		key.UserID,
		key.ChannelID,
	).Scan(&conversationID)

	if err != nil {
		if err == pgx.ErrNoRows {
			return []ai.Message{}, nil
		}

		return nil, fmt.Errorf("find active conversation: %w", err)
	}

	rows, err := m.db.Query(
		ctx,
		`
		SELECT role, content
		FROM (
			SELECT role, content, created_at, id
			FROM messages
			WHERE conversation_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		) recent_messages
		ORDER BY created_at ASC, id ASC
		`,
		conversationID,
		m.maxTurns*2,
	)
	if err != nil {
		return nil, fmt.Errorf("get conversation messages: %w", err)
	}
	defer rows.Close()

	messages := make([]ai.Message, 0)

	for rows.Next() {
		var message ai.Message

		if err := rows.Scan(
			&message.Role,
			&message.Content,
		); err != nil {
			return nil, fmt.Errorf("scan conversation message: %w", err)
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversation messages: %w", err)
	}

	return messages, nil
}

// Add menyimpan identitas user, percakapan aktif,
// dan sepasang pesan user-assistant dalam satu transaksi.
func (m *Manager) Add(
	ctx context.Context,
	key SessionKey,
	userMessage string,
	assistantMessage string,
) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin conversation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Simpan atau perbarui identitas Discord user.
	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO users (
			discord_user_id,
			username,
			global_name
		)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (discord_user_id)
		DO UPDATE SET
			username = EXCLUDED.username,
			global_name = EXCLUDED.global_name,
			updated_at = NOW()
		`,
		key.UserID,
		key.Username,
		key.GlobalName,
	)
	if err != nil {
		return fmt.Errorf("upsert Discord user: %w", err)
	}

	// Ambil percakapan aktif atau buat yang baru.
	var conversationID int64

	err = tx.QueryRow(
		ctx,
		`
		INSERT INTO conversations (
			discord_user_id,
			channel_id,
			guild_id,
			status
		)
		VALUES ($1, $2, NULLIF($3, ''), 'active')
		ON CONFLICT (discord_user_id, channel_id)
		WHERE status = 'active'
		DO UPDATE SET
			guild_id = EXCLUDED.guild_id,
			updated_at = NOW()
		RETURNING id
		`,
		key.UserID,
		key.ChannelID,
		key.GuildID,
	).Scan(&conversationID)
	if err != nil {
		return fmt.Errorf("upsert active conversation: %w", err)
	}

	// Simpan pesan user.
	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO messages (
			conversation_id,
			role,
			content
		)
		VALUES ($1, 'user', $2)
		`,
		conversationID,
		userMessage,
	)
	if err != nil {
		return fmt.Errorf("save user message: %w", err)
	}

	// Simpan jawaban final assistant.
	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO messages (
			conversation_id,
			role,
			content
		)
		VALUES ($1, 'assistant', $2)
		`,
		conversationID,
		assistantMessage,
	)
	if err != nil {
		return fmt.Errorf("save assistant message: %w", err)
	}

	// Perbarui waktu aktivitas percakapan.
	_, err = tx.Exec(
		ctx,
		`
		UPDATE conversations
		SET
			last_message_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
		`,
		conversationID,
	)
	if err != nil {
		return fmt.Errorf("update conversation timestamp: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit conversation transaction: %w", err)
	}

	return nil
}

// Reset mengarsipkan percakapan aktif.
// Riwayat lama tetap tersimpan di database.
func (m *Manager) Reset(
	ctx context.Context,
	key SessionKey,
) error {
	_, err := m.db.Exec(
		ctx,
		`
		UPDATE conversations
		SET
			status = 'archived',
			updated_at = NOW()
		WHERE discord_user_id = $1
		  AND channel_id = $2
		  AND status = 'active'
		`,
		key.UserID,
		key.ChannelID,
	)
	if err != nil {
		return fmt.Errorf("archive active conversation: %w", err)
	}

	return nil
}
