package auction_butler

import (
	"fmt"
	"math"
	"strings"
	"time"

	"errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/telegram-bot-api.v4"
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
	rescheduleChan         chan int
	lastBidMessage         *Context
	auctionEndTime         time.Time
	runningCountDown       bool
	bidChan                chan int
}

type Context struct {
	message *tgbotapi.Message
	User    *User
}

type Bid struct {
	// amount
	Value float64
	// btc/sky
	CoinType string
}

type CommandHandler func(*Bot, *Context, string, string) error
type MessageHandler func(*Bot, *Context, string) (bool, error)

func (b *Bid) String() string {
	return fmt.Sprintf("%v %v", b.Value, b.CoinType)
}

func (b *Bid) Convert(conversionFactor int64) string {
	if b.CoinType == "SKY" {
		return fmt.Sprintf("%.2f %v", b.Value/float64(conversionFactor), "BTC")
	}

	return fmt.Sprintf("%v %v", math.Ceil(b.Value*float64(conversionFactor)), "SKY")
}

func (bot *Bot) enableUser(u *User) ([]string, error) {
	var actions []string
	if !u.Exists() {
		actions = append(actions, "created")
	}
	if u.Banned {
		u.Banned = false
		actions = append(actions, "unbanned")
	}
	if !u.Enlisted {
		u.Enlisted = true
		actions = append(actions, "enlisted")
	}
	if len(actions) > 0 {
		if err := bot.db.PutUser(u); err != nil {
			return nil, fmt.Errorf("failed to change user status: %v", err)
		}
	}
	return actions, nil
}

func (bot *Bot) enableUserVerbosely(ctx *Context, dbuser *User) error {
	actions, err := bot.enableUser(dbuser)
	if err != nil {
		return fmt.Errorf("failed to enable user: %v", err)
	}
	if len(actions) > 0 {
		return bot.Reply(ctx, strings.Join(actions, ", "))
	}
	return bot.Reply(ctx, "no action required")
}

func (bot *Bot) handleForwardedMessageFrom(ctx *Context, id int) error {
	args := tgbotapi.ChatConfigWithUser{bot.config.ChatID, "", id}
	member, err := bot.telegram.GetChatMember(args)
	if err != nil {
		return fmt.Errorf("failed to get chat member from telegram: %v", err)
	}

	if !member.IsMember() && !member.IsCreator() && !member.IsAdministrator() {
		return bot.Reply(ctx, "that user is not a member of the chat")
	}

	user := member.User
	log.Printf("forwarded from user: %#v", user)
	dbuser := bot.db.GetUser(user.ID)
	if dbuser == nil {
		dbuser = &User{
			ID:        user.ID,
			UserName:  user.UserName,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		}
	}

	return bot.enableUserVerbosely(ctx, dbuser)
}

func (bot *Bot) handleCommand(ctx *Context, command, args string) error {
	if !ctx.User.Banned {
		handler, found := bot.commandHandlers[command]
		if found {
			return handler(bot, ctx, command, args)
		}
	}

	if ctx.User.Admin {
		handler, found := bot.adminCommandHandlers[command]
		if found {
			return handler(bot, ctx, command, args)
		}
	}

	return fmt.Errorf("command not found: %s", command)
}

func (bot *Bot) handlePrivateMessage(ctx *Context) error {
	if ctx.User.Admin {
		// let admin force add users by forwarding their messages
		if u := ctx.message.ForwardFrom; u != nil {
			if err := bot.handleForwardedMessageFrom(ctx, u.ID); err != nil {
				return fmt.Errorf("failed to add user %s: %v", u.String(), err)
			}
			return nil
		}
	}

	if ctx.message.IsCommand() {
		cmd, args := ctx.message.Command(), ctx.message.CommandArguments()
		err := bot.handleCommand(ctx, cmd, args)
		if err != nil {
			log.Printf("command '/%s %s' failed: %v", cmd, args, err)
			return bot.Reply(ctx, fmt.Sprintf("command failed: %v", err))
		}
		return nil
	}

	for i := len(bot.privateMessageHandlers) - 1; i >= 0; i-- {
		handler := bot.privateMessageHandlers[i]
		next, err := handler(bot, ctx, ctx.message.Text)
		if err != nil {
			return fmt.Errorf("private message handler failed: %v", err)
		}
		if !next {
			break
		}
	}

	return nil
}

func (bot *Bot) handleUserJoin(ctx *Context, user *tgbotapi.User) error {
	if user.ID == bot.telegram.Self.ID {
		log.Printf("i have joined the group")
		return nil
	}
	dbuser := bot.db.GetUser(user.ID)
	if dbuser == nil {
		dbuser = &User{
			ID:        user.ID,
			UserName:  user.UserName,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		}
	}
	dbuser.Enlisted = true
	if err := bot.db.PutUser(dbuser); err != nil {
		log.Printf("failed to save the user")
		return err
	}

	log.Printf("user joined: %s", dbuser.NameAndTags())
	msg, err := bot.Send(ctx, "reply", "html", `Welcome to the KittyCash Auction group. Please familiarise yourself with the rules in the <a href="t.me/KittyCashAuction/746">pinned message</a> before bidding on a Legendary Kitty.`)

	if err != nil {
		return err
	}

	go func() {
		time.Sleep(bot.config.MsgDeleteCounter.Duration)
		bot.telegram.DeleteMessage(tgbotapi.DeleteMessageConfig{
			ChatID:    bot.config.ChatID,
			MessageID: msg.MessageID,
		})
	}()

	return nil
}

func (bot *Bot) handleUserLeft(ctx *Context, user *tgbotapi.User) error {
	if user.ID == bot.telegram.Self.ID {
		log.Printf("i have left the group")
		return nil
	}
	dbuser := bot.db.GetUser(user.ID)
	if dbuser != nil {
		dbuser.Enlisted = false
		if err := bot.db.PutUser(dbuser); err != nil {
			log.Printf("failed to save the user")
			return err
		}

		log.Printf("user left: %s", dbuser.NameAndTags())
	}
	return nil
}

func (bot *Bot) DeleteMsg(chatID int64, msgID int) {
	bot.telegram.DeleteMessage(tgbotapi.DeleteMessageConfig{
		bot.config.ChatID,
		msgID,
	})
}

func (bot *Bot) removeMyName(text string) (string, bool) {
	var removed bool
	var words []string
	for _, word := range strings.Fields(text) {
		if word == "@"+bot.telegram.Self.UserName {
			removed = true
			continue
		}
		words = append(words, word)
	}
	return strings.Join(words, " "), removed
}

func (bot *Bot) isReplyToMe(ctx *Context) bool {
	if re := ctx.message.ReplyToMessage; re != nil {
		if u := re.From; u != nil {
			if u.ID == bot.telegram.Self.ID {
				return true
			}
		}
	}
	return false
}

func (bot *Bot) handleGroupMessage(ctx *Context) error {
	var gerr error
	if u := ctx.message.NewChatMembers; u != nil {
		for _, user := range *u {
			if err := bot.handleUserJoin(ctx, &user); err != nil {
				gerr = err
			}
		}
	}
	if u := ctx.message.LeftChatMember; u != nil {
		if err := bot.handleUserLeft(ctx, u); err != nil {
			gerr = err
		}
	}

	if ctx.User != nil {
		bid, err := findBid(ctx.message.Text)

		//TODO (therealssj): return msgs based on the err returned
		if err != nil {
			if err == ErrNoBidFound && !ctx.User.Admin {
				bot.DeleteMsg(bot.config.ChatID, ctx.message.MessageID)
			}
			return err
		}

		auction := bot.db.GetCurrentAuction()
		if bot.runningCountDown {
			if auction == nil {
				if !ctx.User.Admin {
					bot.DeleteMsg(bot.config.ChatID, ctx.message.MessageID)
				}
				return errors.New("No ongoing auction")
			}
		}
		if bid.CoinType == auction.BidType {
			if bid.Value <= auction.BidVal {
				if !ctx.User.Admin {
					bot.DeleteMsg(bot.config.ChatID, ctx.message.MessageID)
				}
				return fmt.Errorf("bid not more than last bid of %v", auction.BidVal)
			}
		} else {
			switch bid.CoinType {
			case "BTC":
				if bid.Value*float64(bot.config.ConversionFactor) <= auction.BidVal {
					if !ctx.User.Admin {
						bot.DeleteMsg(bot.config.ChatID, ctx.message.MessageID)
					}
					return errors.New("bid less than last bid")

				}
			case "SKY":
				if bid.Value/float64(bot.config.ConversionFactor) <= auction.BidVal {
					if !ctx.User.Admin {
						bot.DeleteMsg(bot.config.ChatID, ctx.message.MessageID)
					}
					return errors.New("bid less than last bid")
				}
			}
		}
		bot.db.SetAuctionBid(auction.ID, bid)
		bot.bidChan <- 1

		//TODO (therealssj): add something to retry sending?
		msg, _ := bot.Send(ctx, "yell", "html", fmt.Sprintf(`<b>Current bid of %v/%v</b>

Bids only please.`, bid.String(), bid.Convert(bot.config.ConversionFactor)))

		if bot.lastBidMessage != nil {
			bot.DeleteMsg(bot.config.ChatID, bot.lastBidMessage.message.MessageID)
		}

		bot.lastBidMessage = &Context{
			message: msg,
			User:    ctx.User,
		}

	}

	return gerr
}

func (bot *Bot) Send(ctx *Context, mode, format, text string) (*tgbotapi.Message, error) {
	var msg tgbotapi.MessageConfig
	switch mode {
	case "whisper":
		msg = tgbotapi.NewMessage(int64(ctx.message.From.ID), text)
	case "reply":
		msg = tgbotapi.NewMessage(ctx.message.Chat.ID, text)
		msg.ReplyToMessageID = ctx.message.MessageID
	case "yell":
		msg = tgbotapi.NewMessage(bot.config.ChatID, text)
	default:
		return nil, fmt.Errorf("unsupported message mode: %s", mode)
	}
	switch format {
	case "markdown":
		msg.ParseMode = "Markdown"
	case "html":
		msg.ParseMode = "HTML"
	case "text":
		msg.ParseMode = ""
	default:
		return nil, fmt.Errorf("unsupported message format: %s", format)
	}

	sentMsg, err := bot.telegram.Send(msg)
	return &sentMsg, err
}

func (bot *Bot) Ask(ctx *Context, text string) error {
	msg := tgbotapi.NewMessage(ctx.message.Chat.ID, text)
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}
	msg.ReplyToMessageID = ctx.message.MessageID
	_, err := bot.telegram.Send(msg)
	return err
}

func (bot *Bot) Reply(ctx *Context, text string) error {
	_, err := bot.Send(ctx, "reply", "text", text)
	return err
}

func (bot *Bot) handleMessage(ctx *Context) error {
	if (ctx.message.Chat.IsGroup() || ctx.message.Chat.IsSuperGroup()) && ctx.message.Chat.ID == bot.config.ChatID {
		return bot.handleGroupMessage(ctx)
	} else if ctx.message.Chat.IsPrivate() {
		return bot.handlePrivateMessage(ctx)
	} else {
		log.Printf("unknown chat %d (%s)", ctx.message.Chat.ID, ctx.message.Chat.UserName)
		return nil
	}
}

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

	bot.setCommandHandlers()

	return &bot, nil
}

func (bot *Bot) handleUpdate(update *tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}
	ctx := Context{message: update.Message}
	if u := ctx.message.From; u != nil {
		dbuser := bot.db.GetUser(u.ID)
		if dbuser == nil {
			member, err := bot.telegram.GetChatMember(tgbotapi.ChatConfigWithUser{
				ChatID: ctx.message.Chat.ID,
				UserID: u.ID,
			})
			if err != nil {
				return fmt.Errorf("unable to fetch chat member %v", u.UserName)
			}
			admin := member.IsAdministrator() || member.IsCreator()
			log.Printf("message from untracked user: %s", u.String())
			if (ctx.message.Chat.IsGroup() || ctx.message.Chat.IsSuperGroup()) && ctx.message.Chat.ID == bot.config.ChatID {
				dbuser = &User{
					ID:        u.ID,
					UserName:  u.UserName,
					FirstName: u.FirstName,
					LastName:  u.LastName,
					Admin:     admin,
				}
				if err := bot.db.PutUser(dbuser); err != nil {
					return fmt.Errorf("failed to save the user: %v", err)
				}
			} else {
				return bot.Reply(&ctx, "Please join the kittycash auction group.")
			}
		}
		ctx.User = dbuser
	}

	return bot.handleMessage(&ctx)
}

func (bot *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10

	//curAuction := bot.db.GetCurrentAuction()
	//
	//if curAuction != nil && curAuction.MessageID != 0 {
	//	bot.lastBidMessage = &Context{
	//		message: &tgbotapi.Message{
	//			MessageID: curAuction.MessageID,
	//		},
	//		User: &User{},
	//	}
	//}

	updates, err := bot.telegram.GetUpdatesChan(u)
	if err != nil {
		return fmt.Errorf("failed to create telegram updates channel: %v", err)
	}

	if err != nil {
		return fmt.Errorf("invalid bot msg announce interval: %v", err)
	}

	go bot.maintain()
	for update := range updates {
		if err := bot.handleUpdate(&update); err != nil {
			log.Printf("error: %v", err)
		}
	}

	close(bot.bidChan)
	log.Printf("stopped")
	return nil
}
