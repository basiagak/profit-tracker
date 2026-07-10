// Package telegram implements the bot command dispatcher and command
// handlers (/start, /ingredient, /item, /purchase, /sale, /report).
package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/repository"
)

// CommandContext carries per-invocation state into a Command's Handler: the
// auto-provisioned shop owner (FR-022), the raw argument string, and the
// underlying Telegram message.
type CommandContext struct {
	User    *domain.User
	Args    string
	Message *tgbotapi.Message
}

// CommandHandler executes one bot command and returns the reply text, or an
// error whose message is sent back to the user as-is (e.g. a validation
// error or usage hint).
type CommandHandler func(ctx *CommandContext) (string, error)

// Command is one registered bot command.
type Command struct {
	Name        string // without the leading "/"
	Usage       string
	Description string
	Handler     CommandHandler
}

// Bot dispatches incoming Telegram updates to registered Commands. Every
// command's data access is scoped to the sender's auto-provisioned users.id
// (FR-002), resolved once per update before dispatch.
type Bot struct {
	api      *tgbotapi.BotAPI
	users    *repository.UserRepository
	commands map[string]*Command
	order    []string
}

// NewBot builds a Bot and registers /start and /help.
func NewBot(token string, users *repository.UserRepository) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	b := &Bot{api: api, users: users, commands: map[string]*Command{}}

	b.Register(&Command{
		Name:        "start",
		Usage:       "/start",
		Description: "Set up your shop and show available commands",
		Handler:     b.handleStart,
	})
	b.Register(&Command{
		Name:        "help",
		Usage:       "/help [command]",
		Description: "List commands, or show usage for one command",
		Handler:     b.handleHelp,
	})

	return b, nil
}

// Register adds a command to the dispatcher (or replaces one of the same
// name). Later phases register /ingredient, /item, /purchase, /sale,
// /report here.
func (b *Bot) Register(cmd *Command) {
	if _, exists := b.commands[cmd.Name]; !exists {
		b.order = append(b.order, cmd.Name)
	}
	b.commands[cmd.Name] = cmd
}

// Run polls for updates via long polling until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			b.handleUpdate(update)
		}
	}
}

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	msg := update.Message
	if msg == nil || !msg.IsCommand() || msg.From == nil {
		return
	}

	var username *string
	if msg.From.UserName != "" {
		u := msg.From.UserName
		username = &u
	}
	var displayName *string
	if name := strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName); name != "" {
		displayName = &name
	}

	user, err := b.users.FindOrCreateByTelegramID(msg.From.ID, username, displayName)
	if err != nil {
		log.Printf("telegram: find-or-create user failed: %v", err)
		b.reply(msg.Chat.ID, "Something went wrong, please try again.")
		return
	}

	cmd, ok := b.commands[msg.Command()]
	if !ok {
		b.reply(msg.Chat.ID, fmt.Sprintf("Unknown command /%s. Send /help to see available commands.", msg.Command()))
		return
	}

	reply, err := cmd.Handler(&CommandContext{User: user, Args: msg.CommandArguments(), Message: msg})
	if err != nil {
		b.reply(msg.Chat.ID, err.Error())
		return
	}
	b.reply(msg.Chat.ID, reply)
}

func (b *Bot) reply(chatID int64, text string) {
	if _, err := b.api.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		log.Printf("telegram: send failed: %v", err)
	}
}

func (b *Bot) handleStart(ctx *CommandContext) (string, error) {
	greeting := "Welcome"
	if ctx.User.DisplayName != nil && *ctx.User.DisplayName != "" {
		greeting = fmt.Sprintf("Welcome, %s", *ctx.User.DisplayName)
	}

	var lines []string
	lines = append(lines, greeting+"! Your shop is ready.", "", "Available commands:")
	lines = append(lines, b.commandList()...)
	return strings.Join(lines, "\n"), nil
}

func (b *Bot) handleHelp(ctx *CommandContext) (string, error) {
	name := strings.TrimPrefix(strings.TrimSpace(ctx.Args), "/")
	if name == "" {
		lines := append([]string{"Available commands:"}, b.commandList()...)
		return strings.Join(lines, "\n"), nil
	}

	cmd, ok := b.commands[name]
	if !ok {
		return fmt.Sprintf("Unknown command /%s.", name), nil
	}
	return fmt.Sprintf("%s\nUsage: %s", cmd.Description, cmd.Usage), nil
}

func (b *Bot) commandList() []string {
	lines := make([]string, 0, len(b.order))
	for _, name := range b.order {
		cmd := b.commands[name]
		lines = append(lines, fmt.Sprintf("/%s - %s", cmd.Name, cmd.Description))
	}
	return lines
}
