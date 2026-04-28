package models

import "time"

// PermissionLevel defines RBAC levels for bot commands.
type PermissionLevel int

const (
	PermissionViewer    PermissionLevel = 1 // read-only
	PermissionEditor    PermissionLevel = 2 // can manage drafts
	PermissionCrawler   PermissionLevel = 3 // can trigger crawls
	PermissionModerator PermissionLevel = 4 // can blacklist
	PermissionAdmin     PermissionLevel = 5 // full access
)

// String returns the string representation of a permission level.
func (p PermissionLevel) String() string {
	switch p {
	case PermissionViewer:
		return "viewer"
	case PermissionEditor:
		return "editor"
	case PermissionCrawler:
		return "crawler"
	case PermissionModerator:
		return "moderator"
	case PermissionAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

// PlatformUpdate represents an incoming message/event from Discord or Telegram.
type PlatformUpdate struct {
	Platform       string            `json:"platform"` // "discord" or "telegram"
	UserID         string            `json:"user_id"`  // platform user ID
	Username       string            `json:"username,omitempty"`
	DisplayName    string            `json:"display_name,omitempty"`
	ConversationID string            `json:"conversation_id"` // channel_id / chat_id
	MessageID      string            `json:"message_id,omitempty"`
	Command        string            `json:"command"`                 // parsed command name
	Args           []string          `json:"args,omitempty"`          // command arguments
	RawText        string            `json:"raw_text"`                // original message text
	IsCommand      bool              `json:"is_command"`              // true if starts with /
	IsCallback     bool              `json:"is_callback,omitempty"`   // Telegram callback query
	CallbackData   string            `json:"callback_data,omitempty"` // Telegram callback payload
	ReplyToID      string            `json:"reply_to_id,omitempty"`   // for threaded replies
	Timestamp      time.Time         `json:"timestamp"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// CommandEntry describes a registered bot command for the help system.
type CommandEntry struct {
	Name        string          `json:"name"` // e.g. "rss add", "crawl start"
	Aliases     []string        `json:"aliases,omitempty"`
	Description string          `json:"description"`        // one-line summary
	Usage       string          `json:"usage"`              // e.g. "/rss add <url>"
	Category    string          `json:"category"`           // "rss", "crawl", "trending", "stats", "system", "draft"
	Permission  PermissionLevel `json:"permission"`         // minimum permission required
	Hidden      bool            `json:"hidden"`             // don't show in /help
	Cooldown    time.Duration   `json:"cooldown,omitempty"` // per-user cooldown
}

// CommandRegistry is the in-memory registry of all bot commands.
type CommandRegistry struct {
	Entries    map[string]*CommandEntry // primary key: canonical name
	Aliases    map[string]string        // alias → canonical name
	ByCategory map[string][]*CommandEntry
}

// NewCommandRegistry creates an empty command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		Entries:    make(map[string]*CommandEntry),
		Aliases:    make(map[string]string),
		ByCategory: make(map[string][]*CommandEntry),
	}
}

// Register adds a command entry and its aliases to the registry.
func (r *CommandRegistry) Register(entry *CommandEntry) {
	r.Entries[entry.Name] = entry
	for _, alias := range entry.Aliases {
		r.Aliases[alias] = entry.Name
	}
	r.ByCategory[entry.Category] = append(r.ByCategory[entry.Category], entry)
}

// Resolve returns the canonical command name, following aliases.
func (r *CommandRegistry) Resolve(name string) string {
	if canonical, ok := r.Aliases[name]; ok {
		return canonical
	}
	return name
}

// Get returns the command entry by canonical name.
func (r *CommandRegistry) Get(name string) *CommandEntry {
	return r.Entries[name]
}

// GetByInput returns the command entry matching the raw input string (command + args).
func (r *CommandRegistry) GetByInput(text string) (string, []string, *CommandEntry) {
	text = trimPrefix(text, "/")
	parts := splitCommand(text)
	if len(parts) == 0 {
		return "", nil, nil
	}

	canonical := r.Resolve(parts[0])
	entry := r.Get(canonical)
	if entry == nil {
		return "", nil, nil
	}

	return canonical, parts[1:], entry
}

// trimPrefix trims a prefix rune if present.
func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// splitCommand splits "command arg1 arg2" into ["command", "arg1", "arg2"].
func splitCommand(text string) []string {
	var parts []string
	var current []byte
	inQuote := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '"' || c == '\'' {
			inQuote = !inQuote
			continue
		}
		if c == ' ' && !inQuote {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = nil
			}
			continue
		}
		current = append(current, c)
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}
