// Package services implements the core business logic for the bot-service.
package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"erg.ninja/internal/modules/bot/commands"
	"erg.ninja/internal/modules/bot/middleware"
	"erg.ninja/internal/modules/bot/models"
	"erg.ninja/internal/modules/bot/platform"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
)

var (
	// ErrUnknownCommand is returned when no handler matches the command.
	ErrUnknownCommand = errors.New("command: unknown command")
	// ErrConversationNotFound is returned when no active conversation exists.
	ErrConversationNotFound = errors.New("conversation: not found")
)

// CommandHandler is the core command dispatch service.
type CommandHandler struct {
	registry   *models.CommandRegistry
	permSvc    *middleware.PermissionService
	convSvc    *ConversationService
	eventBus   *event.EventBus
	workflow   *WorkflowEngine
	discord    *platform.DiscordClient
	telegram   *platform.TelegramClient
	log        *logger.Logger
	cooldowns  map[string]time.Time
	cooldownMu sync.RWMutex
}

// CommandHandlerOption configures a CommandHandler.
type CommandHandlerOption func(*CommandHandler)

// WithCommandHandlerLogger sets the logger.
func WithCommandHandlerLogger(log *logger.Logger) CommandHandlerOption {
	return func(s *CommandHandler) {
		s.log = log
	}
}

// NewCommandHandler creates a new CommandHandler with all dependencies.
func NewCommandHandler(
	permSvc *middleware.PermissionService,
	convSvc *ConversationService,
	eventBus *event.EventBus,
	workflow *WorkflowEngine,
	discord *platform.DiscordClient,
	telegram *platform.TelegramClient,
	opts ...CommandHandlerOption,
) *CommandHandler {
	h := &CommandHandler{
		registry:  commands.BuildRegistry(),
		permSvc:   permSvc,
		convSvc:   convSvc,
		eventBus:  eventBus,
		workflow:  workflow,
		discord:   discord,
		telegram:  telegram,
		log:       logger.NoOp(),
		cooldowns: make(map[string]time.Time),
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Handle processes a platform update and returns the response string.
func (h *CommandHandler) Handle(ctx context.Context, update *models.PlatformUpdate) string {
	if update == nil {
		return "Invalid update received."
	}

	if update.IsCommand {
		return h.handleCommand(ctx, update)
	}

	if h.convSvc != nil {
		wizResp, wizardActive, err := h.convSvc.HandleWizardInput(ctx, update)
		if err != nil {
			h.log.ErrorContext(ctx).Err(err).Msg("conversation: wizard input error")
			return fmt.Sprintf("Wizard error: %v", err)
		}
		if wizardActive && wizResp != "" {
			return wizResp
		}
	}

	return h.buildHelpHint(update)
}

// handleCommand dispatches a command to its handler after permission + cooldown checks.
func (h *CommandHandler) handleCommand(ctx context.Context, update *models.PlatformUpdate) string {
	canonical, args, entry := h.registry.GetByInput(update.Command)
	if entry == nil {
		return h.unknownCommandResponse(update.Command)
	}

	if h.permSvc != nil {
		if err := h.permSvc.Check(ctx, update.UserID, canonical); err != nil {
			h.log.WarnContext(ctx).
				Str("user_id", update.UserID).
				Str("command", canonical).
				Msg("permission denied")
			return fmt.Sprintf("Permission denied: %s", err.Error())
		}
	}

	if entry.Cooldown > 0 {
		if !h.checkCooldown(update.UserID, canonical, entry.Cooldown) {
			return "Command on cooldown. Try again in a few seconds."
		}
	}

	start := time.Now()
	response := h.executeHandler(ctx, canonical, args, update)
	duration := time.Since(start)

	h.log.InfoContext(ctx).
		Str("command", canonical).
		Str("user_id", update.UserID).
		Str("platform", update.Platform).
		Dur("duration", duration).
		Msg("command executed")

	if h.eventBus != nil {
		_ = h.eventBus.Publish(ctx, "bot.command.executed", map[string]any{
			"command":   canonical,
			"user_id":   update.UserID,
			"platform":  update.Platform,
			"duration":  duration.String(),
			"timestamp": time.Now(),
		})
	}

	return response
}

// executeHandler routes to the appropriate command handler function.
func (h *CommandHandler) executeHandler(ctx context.Context, cmd string, args []string, update *models.PlatformUpdate) string {
	switch cmd {
	case "rss add":
		return commands.HandleRSSAdd(ctx, args, update)
	case "rss list":
		return commands.HandleRSSList(ctx, args, update)
	case "rss remove":
		return commands.HandleRSSRemove(ctx, args, update)
	case "rss sync":
		return commands.HandleRSSSync(ctx, args, update)
	case "rss preview":
		return commands.HandleRSSPreview(ctx, args, update)
	case "crawl start":
		return commands.HandleCrawlStart(ctx, args, update)
	case "crawl status":
		return commands.HandleCrawlStatus(ctx, args, update)
	case "crawl stop":
		return commands.HandleCrawlStop(ctx, args, update)
	case "crawl batch":
		return commands.HandleCrawlBatch(ctx, args, update)
	case "crawl history":
		return commands.HandleCrawlHistory(ctx, args, update)
	case "trending top":
		return commands.HandleTrendingTop(ctx, args, update)
	case "trending keyword":
		return commands.HandleTrendingKeyword(ctx, args, update)
	case "draft list":
		return commands.HandleDraftList(ctx, args, update)
	case "draft publish":
		return commands.HandleDraftPublish(ctx, args, update)
	case "draft delete":
		return commands.HandleDraftDelete(ctx, args, update)
	case "stats users":
		return commands.HandleStatsUsers(ctx, args, update)
	case "stats crawler":
		return commands.HandleStatsCrawler(ctx, args, update)
	case "stats queue":
		return commands.HandleStatsQueue(ctx, args, update)
	case "stats system":
		return commands.HandleStatsSystem(ctx, args, update)
	case "system health":
		return commands.HandleSystemHealth(ctx, args, update)
	case "system ping":
		return commands.HandleSystemPing(ctx, args, update)
	case "system reload":
		return commands.HandleSystemReload(ctx, args, update)
	case "system version":
		return commands.HandleSystemVersion(ctx, args, update)
	case "help":
		return h.buildHelp(update)
	default:
		return h.unknownCommandResponse(cmd)
	}
}

func (h *CommandHandler) checkCooldown(userID, command string, d time.Duration) bool {
	h.cooldownMu.Lock()
	defer h.cooldownMu.Unlock()
	key := userID + ":" + command
	last, ok := h.cooldowns[key]
	if ok && time.Since(last) < d {
		return false
	}
	h.cooldowns[key] = time.Now()
	return true
}

func (h *CommandHandler) unknownCommandResponse(cmd string) string {
	return fmt.Sprintf("Unknown command: %s\n\nType /help to see available commands.", cmd)
}

func (h *CommandHandler) buildHelpHint(update *models.PlatformUpdate) string {
	switch update.Platform {
	case "telegram":
		return "Hi! Type /help to see available commands."
	case "discord":
		return "Hi! Type /help to see available commands."
	default:
		return "Hi! Type /help to see available commands."
	}
}

func (h *CommandHandler) buildHelp(update *models.PlatformUpdate) string {
	var lines []string
	lines = append(lines, "Available Commands")
	lines = append(lines, "")

	categories := []string{"rss", "crawl", "trending", "draft", "stats", "system"}

	for _, cat := range categories {
		entries := h.registry.ByCategory[cat]
		if len(entries) == 0 {
			continue
		}
		lines = append(lines, strings.ToUpper(cat))
		for _, e := range entries {
			if e.Hidden {
				continue
			}
			if h.permSvc != nil {
				required := h.permSvc.GetCommandPermission(e.Name)
				userLvl, err := h.permSvc.GetUserLevel(context.Background(), update.UserID)
				if err == nil && userLvl < required {
					continue
				}
			}
			lines = append(lines, fmt.Sprintf("  %s - %s", e.Usage, e.Description))
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// SendPlatformMessage sends a response back to the user on the appropriate platform.
func (h *CommandHandler) SendPlatformMessage(ctx context.Context, update *models.PlatformUpdate, text string) error {
	switch update.Platform {
	case "discord":
		return h.discord.SendChannelMessage(ctx, update.ConversationID, text)
	case "telegram":
		chatID, _ := strconv.ParseInt(update.ConversationID, 10, 64)
		_, err := h.telegram.SendMessage(ctx, chatID, text)
		return err
	default:
		return fmt.Errorf("unsupported platform: %s", update.Platform)
	}
}
