package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/razenry/mcqueen-ai/internal/ai"
	"github.com/razenry/mcqueen-ai/internal/conversation"
)

type Bot struct {
	session      *discordgo.Session
	provider     ai.Provider
	memory       *conversation.Manager
	systemPrompt string
}

func New(
	token string,
	provider ai.Provider,
	systemPrompt string,
) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	session.Identify.Intents =
		discordgo.IntentsGuilds |
			discordgo.IntentsGuildMessages |
			discordgo.IntentsDirectMessages |
			discordgo.IntentsMessageContent

	bot := &Bot{
		session:      session,
		provider:     provider,
		memory:       conversation.NewManager(6),
		systemPrompt: systemPrompt,
	}

	bot.registerHandlers()

	return bot, nil
}

func (b *Bot) Start() error {
	return b.session.Open()
}

func (b *Bot) Close() error {
	return b.session.Close()
}

func (b *Bot) registerHandlers() {
	b.session.AddHandler(b.handleReady)
	b.session.AddHandler(b.handleMessage)
}

func (b *Bot) handleReady(
	s *discordgo.Session,
	event *discordgo.Ready,
) {
	fmt.Printf(
		"McQueen is online as %s\n",
		event.User.Username,
	)
}
