package discord

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/razenry/mcqueen-ai/internal/ai"
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

	isMention := strings.Contains(content, "<@"+botID+">") ||
		strings.Contains(content, "<@!"+botID+">")

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
		switch strings.ToLower(input) {
		case "help":
			b.sendReply(s, message,
				"**McQueen Commands**\n"+
					"`!mcqueen help` - Bantuan\n"+
					"`!mcqueen reset` - Reset memory\n"+
					"Reply pesan McQueen untuk mengobrol.")
			return

		case "reset":
			key := message.ChannelID + ":" + message.Author.ID
			b.memory.Reset(key)
			b.sendReply(s, message,
				"Ara ara, percakapan kita telah dimulai kembali, Trainer.")
			return

		case "ping":
			b.sendReply(s, message,
				"Pong! McQueen siap melayani Anda.")
			return
		}
	}

	if input == "" {
		b.sendReply(s, message,
			"Ara ara... ada yang ingin Anda bicarakan, Trainer?")
		return
	}

	b.handleConversation(s, message, input)
}

func (b *Bot) handleConversation(
	s *discordgo.Session,
	message *discordgo.Message,
	input string,
) {
	key := message.ChannelID + ":" + message.Author.ID

	// Show typing indicator while waiting for AI.
	_ = s.ChannelTyping(message.ChannelID)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		60*time.Second,
	)
	defer cancel()

	messages := []ai.Message{
		{
			Role:    "system",
			Content: b.systemPrompt,
		},
	}

	messages = append(
		messages,
		b.memory.Get(key)...,
	)

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: input,
	})

	response, err := b.provider.Chat(ctx, messages)
	if err != nil {
		fmt.Printf("AI conversation error: %v\n", err)

		b.sendReply(s, message,
			"Maaf, Trainer. Sepertinya saya sedang mengalami sedikit kendala. Mohon coba beberapa saat lagi.")
		return
	}

	b.memory.Add(key, input, response)

	b.sendReply(s, message, response)
}

func (b *Bot) sendReply(
	s *discordgo.Session,
	message *discordgo.Message,
	content string,
) {
	// Discord messages have a 2000-character content limit.
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

// Check whether the message replies to McQueen.
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

	// Use the resolved message when Discord provides it.
	referencedMessage := message.ReferencedMessage

	// Fetch the referenced message if it was not resolved.
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

	if referencedMessage.Author == nil {
		return false
	}

	return referencedMessage.Author.ID == s.State.User.ID
}

func (b *Bot) handleCommand(
	s *discordgo.Session,
	message *discordgo.Message,
	input string,
) {
	var response string

	switch strings.ToLower(input) {
	case "":
		response = "Ara ara... apakah ada yang ingin Anda bicarakan, Trainer?"

	case "ping":
		response = "Pong! McQueen siap melayani Anda."

	case "help":
		response = `**Mejiro McQueen Commands**

!mcqueen - Menyapa McQueen
!mcqueen ping - Memeriksa koneksi
!mcqueen help - Melihat bantuan

Fitur AI sedang dalam pengembangan.`

	default:
		response = "Ara ara... fitur percakapan AI saya sedang dipersiapkan, Trainer."
	}

	_, err := s.ChannelMessageSend(
		message.ChannelID,
		response,
	)

	if err != nil {
		// Log error instead of silently ignoring it.
		println("Failed to send Discord message:", err.Error())
	}
}
