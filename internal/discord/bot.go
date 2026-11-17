package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	mcqueenagent "github.com/razenry/mcqueen-ai/internal/agent"
	"github.com/razenry/mcqueen-ai/internal/conversation"
	usermemory "github.com/razenry/mcqueen-ai/internal/memory"
)

type Bot struct {
	session    *discordgo.Session
	agent      *mcqueenagent.Agent
	memory     *conversation.Manager
	userMemory *usermemory.Service
}

func New(
	token string,
	aiAgent *mcqueenagent.Agent,
	memory *conversation.Manager,
	userMemory *usermemory.Service,
) (*Bot, error) {
	if aiAgent == nil {
		return nil, fmt.Errorf("AI agent cannot be nil")
	}

	if memory == nil {
		return nil, fmt.Errorf("conversation manager cannot be nil")
	}

	if userMemory == nil {
		return nil, fmt.Errorf("user memory service cannot be nil")
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create Discord session: %w", err)
	}

	session.Identify.Intents =
		discordgo.IntentsGuilds |
			discordgo.IntentsGuildMessages |
			discordgo.IntentsDirectMessages |
			discordgo.IntentsMessageContent

	bot := &Bot{
		session:    session,
		agent:      aiAgent,
		memory:     memory,
		userMemory: userMemory,
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
