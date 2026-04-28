package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/bot/models"
	"erg.ninja/pkg/logger"
)

var (
	// ErrWorkflowNotFound is returned when a workflow is not found.
	ErrWorkflowNotFound = errors.New("workflow: not found")
	// ErrWorkflowStepNotFound is returned when a step is not found in the workflow.
	ErrWorkflowStepNotFound = errors.New("workflow: step not found")
	// ErrWorkflowTimeout is returned when a workflow step times out.
	ErrWorkflowTimeout = errors.New("workflow: step timeout")
)

// WorkflowEngine executes multi-step workflows with branching, checkpoints,
// and resume-from-failure capabilities.
type WorkflowEngine struct {
	coll     *mongo.Collection
	log      *logger.Logger
	handlers map[string]WorkflowStepHandler // step type → handler function
}

// WorkflowStepHandler is the function signature for executing a workflow step.
type WorkflowStepHandler func(ctx context.Context, step models.WorkflowStep, data map[string]any) (output map[string]any, nextStep string, err error)

// WorkflowEngineOption configures a WorkflowEngine.
type WorkflowEngineOption func(*WorkflowEngine)

// WithWorkflowEngineLogger sets the logger.
func WithWorkflowEngineLogger(log *logger.Logger) WorkflowEngineOption {
	return func(e *WorkflowEngine) {
		e.log = log
	}
}

// NewWorkflowEngine creates a WorkflowEngine with the given MongoDB collection.
func NewWorkflowEngine(coll *mongo.Collection, opts ...WorkflowEngineOption) *WorkflowEngine {
	e := &WorkflowEngine{
		coll:     coll,
		log:      logger.NoOp(),
		handlers: make(map[string]WorkflowStepHandler),
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// RegisterStepHandler registers a handler for a step type.
func (e *WorkflowEngine) RegisterStepHandler(stepType string, handler WorkflowStepHandler) {
	e.handlers[stepType] = handler
}

// StartWorkflow creates and starts a new workflow execution.
func (e *WorkflowEngine) StartWorkflow(ctx context.Context, workflowID, convID, userID string, steps []models.WorkflowStep) (*models.WorkflowExecution, error) {
	if len(steps) == 0 {
		return nil, errors.New("workflow: at least one step is required")
	}

	exec := &models.WorkflowExecution{
		WorkflowID:  workflowID,
		ConvID:      convID,
		UserID:      userID,
		Status:      models.WorkflowRunning,
		CurrentStep: steps[0].Name,
		Steps:       steps,
		Data:        make(map[string]any),
		MaxRetries:  3,
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Persist to MongoDB.
	res, err := e.coll.InsertOne(ctx, exec)
	if err != nil {
		return nil, fmt.Errorf("workflow: insert execution: %w", err)
	}
	exec.ID = res.InsertedID.(bson.ObjectID)

	e.log.InfoContext(ctx).
		Str("workflow_id", workflowID).
		Str("exec_id", exec.ID.Hex()).
		Str("conv_id", convID).
		Msg("workflow started")

	return exec, nil
}

// ResumeWorkflow resumes a paused or failed workflow from its current step.
func (e *WorkflowEngine) ResumeWorkflow(ctx context.Context, execID string) (*models.WorkflowExecution, error) {
	var exec models.WorkflowExecution
	err := e.coll.FindOne(ctx, bson.M{"_id": execID}).Decode(&exec)
	if err != nil {
		return nil, fmt.Errorf("workflow: load execution: %w", err)
	}

	if exec.Status != models.WorkflowPaused && exec.Status != models.WorkflowFailed {
		return nil, fmt.Errorf("workflow: cannot resume: status is %s", exec.Status)
	}

	exec.Status = models.WorkflowRunning
	exec.UpdatedAt = time.Now()
	_, err = e.coll.UpdateOne(ctx, bson.M{"_id": exec.ID}, bson.M{"$set": bson.M{
		"status":     models.WorkflowRunning,
		"updated_at": time.Now(),
	}})
	if err != nil {
		return nil, fmt.Errorf("workflow: resume update: %w", err)
	}

	e.log.InfoContext(ctx).Str("exec_id", execID).Msg("workflow resumed")
	return &exec, nil
}

// ExecuteNextStep runs the current step of a workflow execution.
func (e *WorkflowEngine) ExecuteNextStep(ctx context.Context, execID string, stepInput map[string]any) (*models.WorkflowExecution, error) {
	var exec models.WorkflowExecution
	err := e.coll.FindOne(ctx, bson.M{"_id": execID}).Decode(&exec)
	if err != nil {
		return nil, fmt.Errorf("workflow: load execution: %w", err)
	}

	// Merge step input into execution data.
	for k, v := range stepInput {
		exec.Data[k] = v
	}

	// Find current step.
	var currentStep *models.WorkflowStep
	for i := range exec.Steps {
		if exec.Steps[i].Name == exec.CurrentStep {
			currentStep = &exec.Steps[i]
			break
		}
	}
	if currentStep == nil {
		return nil, ErrWorkflowStepNotFound
	}

	// Execute the step.
	start := time.Now()
	var nextStep string
	var output map[string]any

	if handler, ok := e.handlers[currentStep.Type]; ok && handler != nil {
		var stepErr error
		output, nextStep, stepErr = handler(ctx, *currentStep, exec.Data)
		if stepErr != nil {
			return e.handleStepFailure(ctx, &exec, currentStep, start, stepErr)
		}
	} else {
		// Default: advance to next step.
		nextStep = currentStep.OnSuccess
	}

	// Log step execution.
	stepLog := models.WorkflowStepLog{
		Step:       currentStep.Name,
		Status:     "success",
		ExecutedAt: start,
		Duration:   time.Since(start),
	}
	if output != nil {
		if s, ok := output["input"].(string); ok {
			stepLog.Input = s
		}
		if s, ok := output["output"].(string); ok {
			stepLog.Output = s
		}
	}
	exec.StepHistory = append(exec.StepHistory, stepLog)

	// Check if workflow is complete.
	if nextStep == "" || nextStep == currentStep.OnComplete {
		return e.completeWorkflow(ctx, &exec)
	}

	// Advance to next step.
	exec.CurrentStep = nextStep
	exec.UpdatedAt = time.Now()
	exec.Status = models.WorkflowRunning

	_, err = e.coll.UpdateOne(ctx, bson.M{"_id": exec.ID}, bson.M{"$set": bson.M{
		"current_step": exec.CurrentStep,
		"status":       models.WorkflowRunning,
		"updated_at":   time.Now(),
		"data":         exec.Data,
		"step_history": exec.StepHistory,
	}})
	if err != nil {
		return nil, fmt.Errorf("workflow: advance step: %w", err)
	}

	return &exec, nil
}

// handleStepFailure handles a step failure, potentially retrying or advancing to error branch.
func (e *WorkflowEngine) handleStepFailure(ctx context.Context, exec *models.WorkflowExecution, step *models.WorkflowStep, start time.Time, stepErr error) (*models.WorkflowExecution, error) {
	exec.RetryCount++

	stepLog := models.WorkflowStepLog{
		Step:       step.Name,
		Status:     "fail",
		Error:      stepErr.Error(),
		ExecutedAt: start,
		Duration:   time.Since(start),
	}
	exec.StepHistory = append(exec.StepHistory, stepLog)

	e.log.ErrorContext(ctx).Err(stepErr).
		Str("step", step.Name).
		Int("retry", exec.RetryCount).
		Msg("workflow step failed")

	if exec.RetryCount <= exec.MaxRetries {
		// Retry: try on_fail branch or same step.
		nextStep := step.OnFail
		if nextStep == "" {
			nextStep = step.Name
		}
		exec.CurrentStep = nextStep
		exec.UpdatedAt = time.Now()

		_, err := e.coll.UpdateOne(ctx, bson.M{"_id": exec.ID}, bson.M{"$set": bson.M{
			"current_step": exec.CurrentStep,
			"retry_count":  exec.RetryCount,
			"step_history": exec.StepHistory,
			"updated_at":   time.Now(),
		}})
		if err != nil {
			return nil, fmt.Errorf("workflow: retry update: %w", err)
		}
		return exec, nil
	}

	// Max retries exceeded → mark as failed.
	exec.Status = models.WorkflowFailed
	exec.Error = stepErr.Error()
	exec.CurrentStep = step.OnTimeout // advance to timeout step if defined

	now := time.Now()
	exec.CompletedAt = &now
	exec.UpdatedAt = time.Now()

	_, err := e.coll.UpdateOne(ctx, bson.M{"_id": exec.ID}, bson.M{"$set": bson.M{
		"status":       models.WorkflowFailed,
		"error":        stepErr.Error(),
		"current_step": exec.CurrentStep,
		"retry_count":  exec.RetryCount,
		"step_history": exec.StepHistory,
		"updated_at":   time.Now(),
		"completed_at": now,
	}})
	if err != nil {
		return nil, fmt.Errorf("workflow: mark failed: %w", err)
	}

	return exec, nil
}

// completeWorkflow marks a workflow execution as completed.
func (e *WorkflowEngine) completeWorkflow(ctx context.Context, exec *models.WorkflowExecution) (*models.WorkflowExecution, error) {
	exec.Status = models.WorkflowCompleted
	now := time.Now()
	exec.CompletedAt = &now
	exec.UpdatedAt = time.Now()

	_, err := e.coll.UpdateOne(ctx, bson.M{"_id": exec.ID}, bson.M{"$set": bson.M{
		"status":       models.WorkflowCompleted,
		"updated_at":   time.Now(),
		"completed_at": now,
		"step_history": exec.StepHistory,
	}})
	if err != nil {
		return nil, fmt.Errorf("workflow: complete: %w", err)
	}

	e.log.InfoContext(ctx).Str("exec_id", exec.ID.Hex()).Msg("workflow completed")
	return exec, nil
}

// PauseWorkflow pauses an active workflow execution.
func (e *WorkflowEngine) PauseWorkflow(ctx context.Context, execID string) error {
	filter := bson.M{"_id": execID, "status": models.WorkflowRunning}
	update := bson.M{"$set": bson.M{
		"status":     models.WorkflowPaused,
		"updated_at": time.Now(),
	}}
	res, err := e.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("workflow: pause: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrWorkflowNotFound
	}
	return nil
}

// CancelWorkflow cancels a workflow execution.
func (e *WorkflowEngine) CancelWorkflow(ctx context.Context, execID string) error {
	filter := bson.M{"_id": execID}
	update := bson.M{"$set": bson.M{
		"status":     models.WorkflowCancelled,
		"updated_at": time.Now(),
	}}
	_, err := e.coll.UpdateOne(ctx, filter, update)
	return err
}

// GetWorkflow returns a workflow execution by ID.
func (e *WorkflowEngine) GetWorkflow(ctx context.Context, execID string) (*models.WorkflowExecution, error) {
	var exec models.WorkflowExecution
	err := e.coll.FindOne(ctx, bson.M{"_id": execID}).Decode(&exec)
	if err != nil {
		return nil, fmt.Errorf("workflow: get: %w", err)
	}
	return &exec, nil
}

// ListWorkflows returns all workflow executions for a conversation.
func (e *WorkflowEngine) ListWorkflows(ctx context.Context, convID string, limit int64) ([]*models.WorkflowExecution, error) {
	if limit <= 0 {
		limit = 20
	}
	opts := options.Find().SetSort(bson.D{{Key: "started_at", Value: -1}}).SetLimit(limit)
	cursor, err := e.coll.Find(ctx, bson.M{"conv_id": convID}, opts)
	if err != nil {
		return nil, fmt.Errorf("workflow: list: %w", err)
	}
	defer cursor.Close(ctx)

	var execs []*models.WorkflowExecution
	if err := cursor.All(ctx, &execs); err != nil {
		return nil, fmt.Errorf("workflow: decode list: %w", err)
	}
	return execs, nil
}
