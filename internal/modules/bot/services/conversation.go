package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/bot/cache"
	"erg.ninja/internal/modules/bot/models"
	"erg.ninja/pkg/logger"
)

const (
	wizardTTL      = 5 * time.Minute
	wizardDataSize = 100
)

const wizardTemplateKey = "wizard_template"

// WizardStep defines a named step in a conversation wizard.
type WizardStep struct {
	Name       string             // step identifier
	Prompt     string             // message sent to user
	Validate   func(string) error // optional input validator
	NextStep   string             // next step name on success
	OnComplete string             // final step name → end wizard
	OnTimeout  string             // step name to go to on timeout
}

// ConversationService manages multi-step conversation wizards. It is thread-safe
// and persists wizard state to MongoDB for durability across restarts.
type ConversationService struct {
	coll    *mongo.Collection
	redis   cache.RedisCache
	log     *logger.Logger
	mu      sync.RWMutex
	wizards map[string]*WizardState // platform_conv_id → state
}

// WizardState holds the runtime state of an active conversation wizard.
type WizardState struct {
	ConvID    string
	UserID    string
	Platform  string
	Step      string
	Data      map[string]string
	Steps     []WizardStep
	StartedAt time.Time
	ExpiresAt time.Time
}

// ConversationServiceOption configures a ConversationService.
type ConversationServiceOption func(*ConversationService)

// WithConversationLogger sets the logger.
func WithConversationLogger(log *logger.Logger) ConversationServiceOption {
	return func(s *ConversationService) {
		s.log = log
	}
}

// NewConversationService creates a ConversationService backed by MongoDB and Redis.
func NewConversationService(coll *mongo.Collection, redis cache.RedisCache, opts ...ConversationServiceOption) *ConversationService {
	s := &ConversationService{
		coll:    coll,
		redis:   redis,
		log:     logger.NoOp(),
		wizards: make(map[string]*WizardState),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// StartWizard initiates a new wizard session for a conversation.
func (s *ConversationService) StartWizard(ctx context.Context, convID, userID, platform, firstStep string, steps []WizardStep) (*WizardState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing wizard.
	if existing, ok := s.wizards[convID]; ok && time.Now().Before(existing.ExpiresAt) {
		// Resume existing wizard.
		return existing, nil
	}

	wiz := &WizardState{
		ConvID:    convID,
		UserID:    userID,
		Platform:  platform,
		Step:      firstStep,
		Data:      make(map[string]string),
		Steps:     steps,
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(wizardTTL),
	}
	if templateName := detectWizardTemplate(steps); templateName != "" {
		if wiz.Data == nil {
			wiz.Data = make(map[string]string)
		}
		wiz.Data[wizardTemplateKey] = templateName
	}

	s.wizards[convID] = wiz

	// Persist to MongoDB.
	if err := s.persistWizard(ctx, wiz); err != nil {
		s.log.ErrorContext(ctx).Err(err).Msg("conversation: persist wizard failed")
	}

	return wiz, nil
}

// HandleWizardInput processes user input in the context of an active wizard.
func (s *ConversationService) HandleWizardInput(ctx context.Context, update *models.PlatformUpdate) (response string, active bool, err error) {
	if update == nil {
		return "", false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Try in-memory first.
	wiz, ok := s.wizards[update.ConversationID]
	if !ok {
		// Try MongoDB.
		wiz, err = s.loadWizard(ctx, update.ConversationID)
		if err != nil {
			return "", false, nil // No active wizard.
		}
		s.wizards[update.ConversationID] = wiz
	}

	// Check expiry.
	if time.Now().After(wiz.ExpiresAt) {
		delete(s.wizards, update.ConversationID)
		_ = s.deleteWizard(ctx, update.ConversationID)
		return "⏰ Wizard expired. Please start again.", false, nil
	}

	// Extend TTL on activity.
	wiz.ExpiresAt = time.Now().Add(wizardTTL)

	// Find current step.
	var currentStep *WizardStep
	for i := range wiz.Steps {
		if wiz.Steps[i].Name == wiz.Step {
			currentStep = &wiz.Steps[i]
			break
		}
	}
	if currentStep == nil {
		return "", false, fmt.Errorf("wizard: step %q not found", wiz.Step)
	}

	// Validate input.
	if currentStep.Validate != nil {
		if err := currentStep.Validate(update.RawText); err != nil {
			return fmt.Sprintf("⚠️ %s\n\n%s", err.Error(), currentStep.Prompt), true, nil
		}
	}

	// Save input.
	wiz.Data[wiz.Step] = update.RawText

	// Advance to next step.
	if currentStep.OnComplete != "" && currentStep.OnComplete == wiz.Step {
		// Wizard complete — build result.
		result := s.buildWizardResult(wiz)
		delete(s.wizards, update.ConversationID)
		_ = s.deleteWizard(ctx, update.ConversationID)
		return result, false, nil
	}

	nextStep := currentStep.NextStep
	if nextStep == "" {
		delete(s.wizards, update.ConversationID)
		_ = s.deleteWizard(ctx, update.ConversationID)
		return "✅ Wizard complete.", false, nil
	}

	wiz.Step = nextStep

	// Persist.
	if err := s.persistWizard(ctx, wiz); err != nil {
		s.log.ErrorContext(ctx).Err(err).Msg("conversation: persist wizard failed")
	}

	return currentStep.Prompt, true, nil
}

// CancelWizard terminates an active wizard session.
func (s *ConversationService) CancelWizard(ctx context.Context, convID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.wizards, convID)
	if err := s.deleteWizard(ctx, convID); err != nil {
		return fmt.Errorf("cancel wizard: %w", err)
	}
	return nil
}

// GetActiveWizard returns the active wizard state for a conversation, if any.
func (s *ConversationService) GetActiveWizard(ctx context.Context, convID string) (*WizardState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if wiz, ok := s.wizards[convID]; ok && time.Now().Before(wiz.ExpiresAt) {
		return wiz, true
	}

	// Fallback: check MongoDB.
	wiz, err := s.loadWizard(ctx, convID)
	if err != nil || wiz == nil {
		return nil, false
	}
	if time.Now().After(wiz.ExpiresAt) {
		return nil, false
	}
	return wiz, true
}

// persistWizard upserts the wizard state to MongoDB.
func (s *ConversationService) persistWizard(ctx context.Context, wiz *WizardState) error {
	filter := bson.M{"platform_conv_id": wiz.ConvID}
	update := bson.M{
		"$set": bson.M{
			"state":                        models.StatePending,
			"wizard_step":                  wiz.Step,
			"wizard_data":                  wiz.Data,
			"context." + wizardTemplateKey: detectWizardTemplate(wiz.Steps),
			"user_id":                      wiz.UserID,
			"platform":                     wiz.Platform,
			"updated_at":                   time.Now(),
			"expires_at":                   wiz.ExpiresAt,
		},
		"$setOnInsert": bson.M{
			"created_at": wiz.StartedAt,
		},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := s.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// loadWizard loads wizard state from MongoDB by conversation ID.
func (s *ConversationService) loadWizard(ctx context.Context, convID string) (*WizardState, error) {
	var conv models.BotConversation
	err := s.coll.FindOne(ctx, bson.M{
		"platform_conv_id": convID,
		"state":            models.StatePending,
	}).Decode(&conv)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("conversation: load wizard: %w", err)
	}

	// Rebuild wizard steps from wizard_step field (placeholder — in production,
	// we persist a template key and fall back to inference from known steps.
	steps := restoreWizardSteps(&conv)
	wiz := &WizardState{
		ConvID:    convID,
		UserID:    conv.UserID,
		Platform:  conv.Platform,
		Step:      conv.WizardStep,
		Data:      make(map[string]string),
		Steps:     steps,
		StartedAt: conv.CreatedAt,
		ExpiresAt: conv.ExpiresAt,
	}

	if conv.WizardData != nil {
		for k, v := range conv.WizardData {
			wiz.Data[k] = v
		}
	}

	return wiz, nil
}

// deleteWizard removes wizard state from MongoDB.
func (s *ConversationService) deleteWizard(ctx context.Context, convID string) error {
	_, err := s.coll.UpdateOne(ctx,
		bson.M{"platform_conv_id": convID},
		bson.M{"$set": bson.M{"state": models.StateActive, "wizard_step": ""}},
	)
	return err
}

// buildWizardResult constructs the final result message from wizard data.
func (s *ConversationService) buildWizardResult(wiz *WizardState) string {
	// Generic result — override per wizard type in specific wizard handlers.
	var parts []string
	for step, val := range wiz.Data {
		parts = append(parts, fmt.Sprintf("%s: `%s`", step, val))
	}
	return "✅ Wizard complete!\n" + strings.Join(parts, "\n")
}

// RegisterRSSWizard returns the wizard steps for the RSS add flow.
func RegisterRSSWizard() []WizardStep {
	return []WizardStep{
		{
			Name:     "input_url",
			Prompt:   "📡 Vui lòng nhập URL RSS feed bạn muốn thêm:",
			NextStep: "input_category",
			Validate: validateURL,
		},
		{
			Name:     "input_category",
			Prompt:   "📂 Chọn danh mục (news, tech, education, other):",
			NextStep: "confirm",
			Validate: validateCategory,
		},
		{
			Name:     "confirm",
			Prompt:   "✅ Xác nhận? Gõ *yes* để thêm, *no* để hủy:",
			NextStep: "done",
			Validate: validateYesNo,
		},
		{
			Name:       "done",
			OnComplete: "done",
			Prompt:     "✅ RSS feed đã được thêm thành công!",
		},
	}
}

func detectWizardTemplate(steps []WizardStep) string {
	if sameWizardSteps(steps, RegisterRSSWizard()) {
		return "rss_add"
	}
	return ""
}

func restoreWizardSteps(conv *models.BotConversation) []WizardStep {
	if conv == nil {
		return nil
	}
	template := ""
	if conv.Context != nil {
		if name, ok := conv.Context[wizardTemplateKey].(string); ok {
			template = name
		}
	}
	if template == "" && conv.WizardData != nil {
		template = conv.WizardData[wizardTemplateKey]
	}
	switch template {
	case "rss_add":
		return RegisterRSSWizard()
	}

	// Fallback inference for older persisted conversations.
	stepNames := map[string]struct{}{}
	for k := range conv.WizardData {
		stepNames[k] = struct{}{}
	}
	stepNames[conv.WizardStep] = struct{}{}
	rssWizard := RegisterRSSWizard()
	matched := 0
	for _, step := range rssWizard {
		if _, ok := stepNames[step.Name]; ok {
			matched++
		}
	}
	if matched > 0 {
		return rssWizard
	}
	return nil
}

func sameWizardSteps(a, b []WizardStep) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].NextStep != b[i].NextStep || a[i].OnComplete != b[i].OnComplete {
			return false
		}
	}
	return true
}

// validateURL performs basic URL validation.
func validateURL(input string) error {
	input = strings.TrimSpace(input)
	if len(input) < 5 {
		return errors.New("URL không hợp lệ. Vui lòng nhập URL đầy đủ (ví dụ: https://example.com/feed)")
	}
	if !strings.Contains(input, ".") {
		return errors.New("URL không hợp lệ. Vui lòng nhập URL đầy đủ")
	}
	return nil
}

// validateCategory checks if the input is a valid category.
func validateCategory(input string) error {
	valid := []string{"news", "tech", "education", "other"}
	input = strings.ToLower(strings.TrimSpace(input))
	for _, v := range valid {
		if v == input {
			return nil
		}
	}
	return fmt.Errorf("Danh mục không hợp lệ. Chọn: %s", strings.Join(valid, ", "))
}

// validateYesNo validates yes/no responses.
func validateYesNo(input string) error {
	input = strings.ToLower(strings.TrimSpace(input))
	if input != "yes" && input != "no" {
		return errors.New("Vui lòng trả lời *yes* hoặc *no*")
	}
	return nil
}
