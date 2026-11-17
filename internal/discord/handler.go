package discord

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/razenry/mcqueen-ai/internal/ai"
	"github.com/razenry/mcqueen-ai/internal/conversation"
)

const commandPrefix = "!mcqueen"

func (b *Bot) handleMessage(
	s *discordgo.Session,
	event *discordgo.MessageCreate,
) {
	message := event.Message

	if message == nil || message.Author == nil {
		return
	}

	if message.Author.Bot {
		return
	}

	content := strings.TrimSpace(message.Content)
	botID := s.State.User.ID

	isCommand := strings.HasPrefix(
		strings.ToLower(content),
		commandPrefix,
	)

	isMention := false

	for _, mentionedUser := range message.Mentions {
		if mentionedUser.ID == botID {
			isMention = true
			break
		}
	}

	isReply := b.isReplyToBot(s, message)

	if !isCommand && !isMention && !isReply {
		return
	}

	input := content

	if isCommand {
		input = strings.TrimSpace(
			content[len(commandPrefix):],
		)
	}

	input = strings.ReplaceAll(input, "<@"+botID+">", "")
	input = strings.ReplaceAll(input, "<@!"+botID+">", "")
	input = strings.TrimSpace(input)

	if isCommand {
		fields := strings.Fields(input)

		if len(fields) > 0 {
			switch strings.ToLower(fields[0]) {
			case "help":
				b.sendReply(
					s,
					message,
					"**McQueen Commands**\n"+
						"`!mcqueen help` - Bantuan\n"+
						"`!mcqueen reset` - Reset percakapan\n"+
						"`!mcqueen memory` - Lihat memori\n"+
						"`!mcqueen forget <id>` - Hapus memori\n"+
						"`!mcqueen ping` - Periksa koneksi\n"+
						"Reply pesan McQueen untuk mengobrol.",
				)
				return

			case "reset":
				b.handleReset(s, message)
				return

			case "memory":
				b.handleMemoryList(s, message)
				return

			case "forget":
				if len(fields) != 2 {
					b.sendReply(
						s,
						message,
						"Gunakan `!mcqueen forget <id>` untuk menghapus memori.",
					)
					return
				}

				b.handleMemoryForget(s, message, fields[1])
				return

			case "ping":
				b.sendReply(
					s,
					message,
					"Pong! McQueen siap melayani Anda.",
				)
				return
			}
		}
	}

	if input == "" {
		b.sendReply(
			s,
			message,
			"Ara ara... ada yang ingin Anda bicarakan, Trainer?",
		)
		return
	}

	b.handleConversation(s, message, input)
}

func (b *Bot) handleReset(
	s *discordgo.Session,
	message *discordgo.Message,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := b.memory.Reset(
		ctx,
		newSessionKey(message),
	); err != nil {
		fmt.Printf("Conversation reset error: %v\n", err)

		b.sendReply(
			s,
			message,
			"Maaf, Trainer. Saya gagal mengarsipkan percakapan kita.",
		)
		return
	}

	b.sendReply(
		s,
		message,
		"Ara ara, percakapan kita telah dimulai kembali, Trainer.",
	)
}

func (b *Bot) handleMemoryList(
	s *discordgo.Session,
	message *discordgo.Message,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	memories, err := b.userMemory.List(ctx, message.Author.ID)
	if err != nil {
		fmt.Printf("Memory list error (user=%s): %v\n", message.Author.ID, err)

		b.sendReply(
			s,
			message,
			"Maaf, Trainer. Saya belum bisa membaca memori Anda.",
		)
		return
	}

	if len(memories) == 0 {
		b.sendReply(
			s,
			message,
			"Saya belum menyimpan informasi tentang Anda, Trainer.",
		)
		return
	}

	var builder strings.Builder

	builder.WriteString("**Memori yang saya simpan tentang Anda:**\n\n")

	for _, item := range memories {
		fmt.Fprintf(
			&builder,
			"**ID %d** · `%s`\n%s\n\n",
			item.ID,
			item.Category,
			item.Content,
		)
	}

	builder.WriteString(
		"Gunakan `!mcqueen forget <id>` untuk menonaktifkan memori.",
	)

	b.sendReply(s, message, builder.String())
}

func (b *Bot) handleMemoryForget(
	s *discordgo.Session,
	message *discordgo.Message,
	rawID string,
) {
	memoryID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || memoryID <= 0 {
		b.sendReply(
			s,
			message,
			"ID memori tidak valid. Periksa ID menggunakan `!mcqueen memory`.",
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	err = b.userMemory.Forget(
		ctx,
		message.Author.ID,
		memoryID,
	)
	if err != nil {
		fmt.Printf(
			"Memory forget error (user=%s, memory=%d): %v\n",
			message.Author.ID,
			memoryID,
			err,
		)

		b.sendReply(
			s,
			message,
			"Memori tidak ditemukan atau gagal dihapus.",
		)
		return
	}

	b.sendReply(
		s,
		message,
		"Baik, Trainer. Memori tersebut tidak akan saya gunakan lagi.",
	)
}

func (b *Bot) handleConversation(
	s *discordgo.Session,
	message *discordgo.Message,
	input string,
) {
	key := newSessionKey(message)

	_ = s.ChannelTyping(message.ChannelID)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		60*time.Second,
	)
	defer cancel()

	// 1. Ambil riwayat percakapan.
	history, err := b.memory.Get(ctx, key)
	if err != nil {
		fmt.Printf(
			"Conversation history error (user=%s, channel=%s): %v\n",
			message.Author.ID,
			message.ChannelID,
			err,
		)

		b.sendReply(
			s,
			message,
			"Maaf, Trainer. Saya belum bisa mengambil riwayat percakapan kita.",
		)
		return
	}

	// 2. Ambil memori jangka panjang milik pengguna.
	memoryContext, err := b.userMemory.GetContext(
		ctx,
		message.Author.ID,
		input,
	)
	if err != nil {
		// Memory bersifat tambahan. Kegagalannya tidak menghentikan chat.
		fmt.Printf(
			"User memory retrieval error (user=%s): %v\n",
			message.Author.ID,
			err,
		)
	}

	// 3. Susun konteks AI.
	messages := make([]ai.Message, 0, len(history)+2)

	if memoryContext != "" {
		messages = append(messages, ai.Message{
			Role:    "system",
			Content: memoryContext,
		})
	}

	messages = append(messages, history...)

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: input,
	})

	// 4. Jalankan AI Agent.
	response, err := b.agent.Run(ctx, messages)
	if err != nil {
		fmt.Printf(
			"AI agent error (user=%s, channel=%s): %v\n",
			message.Author.ID,
			message.ChannelID,
			err,
		)

		b.sendReply(
			s,
			message,
			"Maaf, Trainer. Sepertinya saya sedang mengalami sedikit kendala. Mohon coba beberapa saat lagi.",
		)
		return
	}

	// 5. Simpan percakapan.
	if err := b.memory.Add(
		ctx,
		key,
		input,
		response,
	); err != nil {
		fmt.Printf(
			"Conversation persistence error (user=%s, channel=%s): %v\n",
			message.Author.ID,
			message.ChannelID,
			err,
		)
	}

	// 6. Kirim jawaban terlebih dahulu.
	b.sendReply(s, message, response)

	// 7. Pelajari informasi yang mungkin berguna untuk percakapan berikutnya.
	// Menggunakan context terpisah karena context percakapan akan segera berakhir.
	go b.learnFromConversation(
		message.Author.ID,
		input,
		response,
	)
}

func (b *Bot) learnFromConversation(
	userID string,
	userMessage string,
	assistantMessage string,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	if err := b.userMemory.Learn(
		ctx,
		userID,
		userMessage,
		assistantMessage,
	); err != nil {
		fmt.Printf(
			"Memory learning error (user=%s): %v\n",
			userID,
			err,
		)
	}
}

func newSessionKey(message *discordgo.Message) conversation.SessionKey {
	return conversation.SessionKey{
		UserID:     message.Author.ID,
		Username:   message.Author.Username,
		GlobalName: message.Author.GlobalName,
		ChannelID:  message.ChannelID,
		GuildID:    message.GuildID,
	}
}

func (b *Bot) sendReply(
	s *discordgo.Session,
	message *discordgo.Message,
	content string,
) {
	if len([]rune(content)) > 1900 {
		content = string([]rune(content)[:1900]) +
			"\n\n*...pesan dipersingkat.*"
	}

	_, err := s.ChannelMessageSendReply(
		message.ChannelID,
		content,
		&discordgo.MessageReference{
			MessageID: message.ID,
			ChannelID: message.ChannelID,
			GuildID:   message.GuildID,
		},
	)

	if err != nil {
		fmt.Printf("Discord reply error: %v\n", err)
	}
}

func (b *Bot) isReplyToBot(
	s *discordgo.Session,
	message *discordgo.Message,
) bool {
	if message.MessageReference == nil {
		return false
	}

	reference := message.MessageReference

	if reference.MessageID == "" {
		return false
	}

	channelID := reference.ChannelID

	if channelID == "" {
		channelID = message.ChannelID
	}

	referencedMessage := message.ReferencedMessage

	if referencedMessage == nil {
		var err error

		referencedMessage, err = s.ChannelMessage(
			channelID,
			reference.MessageID,
		)

		if err != nil {
			fmt.Printf(
				"Failed to fetch referenced message: %v\n",
				err,
			)
			return false
		}
	}

	if referencedMessage == nil || referencedMessage.Author == nil {
		return false
	}

	return referencedMessage.Author.ID == s.State.User.ID
}
