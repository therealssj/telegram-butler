package auction_butler

import (
	"gopkg.in/telegram-bot-api.v4"
	"fmt"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.New()
)

type Bot struct {
	config                 *Config
	db                     *DB
	telegram               *tgbotapi.BotAPI
	commandHandlers        map[string]CommandHandler
	adminCommandHandlers   map[string]CommandHandler
	privateMessageHandlers []MessageHandler
	groupMessageHandlers   []MessageHandler
}

type Context struct {
	message *tgbotapi.Message
	User    *User
}

type CommandHandler func(*Bot, *Context, string, string) error
type MessageHandler func(*Bot, *Context, string) (bool, error)

func NewBot(config Config) (*Bot, error) {
	var bot = Bot{
		config:               &config,
		commandHandlers:      make(map[string]CommandHandler),
		adminCommandHandlers: make(map[string]CommandHandler),
	}
	var err error

	if bot.db, err = NewDB(&config.Database); err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if bot.telegram, err = tgbotapi.NewBotAPI(config.Token); err != nil {
		return nil, fmt.Errorf("failed to initialize telegram api: %v", err)
	}

	bot.telegram.Debug = config.Debug

	chat, err := bot.telegram.GetChat(tgbotapi.ChatConfig{config.ChatID, ""})
	if err != nil {
		return nil, fmt.Errorf("failed to get chat info from telegram: %v", err)
	}
	if !chat.IsGroup() && !chat.IsSuperGroup() {
		return nil, fmt.Errorf("only group and supergroups are supported")
	}

	log.Printf("user: %d %s", bot.telegram.Self.ID, bot.telegram.Self.UserName)
	log.Printf("chat: %s %d %s", chat.Type, chat.ID, chat.Title)

	//TODO (therealssj): initialize commands here
	
	return &bot, nil
}
