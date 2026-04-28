package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// WorkflowStatus represents the current status of a workflow.
type WorkflowStatus string

const (
	WorkflowPending   WorkflowStatus = "pending"
	WorkflowRunning   WorkflowStatus = "running"
	WorkflowPaused    WorkflowStatus = "paused"
	WorkflowCompleted WorkflowStatus = "completed"
	WorkflowFailed    WorkflowStatus = "failed"
	WorkflowCancelled WorkflowStatus = "cancelled"
)

// WorkflowStep represents a single step within a workflow definition.
type WorkflowStep struct {
	Name       string            `bson:"name"`                  // unique step identifier
	Label      string            `bson:"label,omitempty"`       // human-readable label
	Type       string            `bson:"type"`                  // "input", "action", "condition", "delay", "notify"
	Command    string            `bson:"command,omitempty"`     // bot command to execute
	Handler    string            `bson:"handler,omitempty"`     // internal handler name
	Prompt     string            `bson:"prompt,omitempty"`      // message to send to user
	OnSuccess  string            `bson:"on_success,omitempty"`  // next step name on success
	OnFail     string            `bson:"on_fail,omitempty"`     // next step name on failure
	OnTimeout  string            `bson:"on_timeout,omitempty"`  // step on timeout
	OnComplete string            `bson:"on_complete,omitempty"` // step that marks completion
	TimeoutSec int               `bson:"timeout_sec,omitempty"`
	Metadata   map[string]string `bson:"metadata,omitempty"`
}

// WorkflowExecution tracks the runtime state of an active workflow instance.
type WorkflowExecution struct {
	ID          bson.ObjectID     `bson:"_id,omitempty"`
	WorkflowID  string            `bson:"workflow_id"` // identifies the workflow type/template
	ConvID      string            `bson:"conv_id"`     // platform_conversation_id
	UserID      string            `bson:"user_id"`
	Status      WorkflowStatus    `bson:"status"`
	CurrentStep string            `bson:"current_step"`
	Steps       []WorkflowStep    `bson:"steps"` // workflow definition
	StepHistory []WorkflowStepLog `bson:"step_history,omitempty"`
	Data        map[string]any    `bson:"data,omitempty"` // accumulated step outputs
	RetryCount  int               `bson:"retry_count"`
	MaxRetries  int               `bson:"max_retries"`
	Error       string            `bson:"error,omitempty"`
	StartedAt   time.Time         `bson:"started_at"`
	UpdatedAt   time.Time         `bson:"updated_at"`
	CompletedAt *time.Time        `bson:"completed_at,omitempty"`
}

// WorkflowStepLog records the result of each step execution.
type WorkflowStepLog struct {
	Step       string        `bson:"step"`
	Status     string        `bson:"status"` // "success", "fail", "skipped"
	Input      string        `bson:"input,omitempty"`
	Output     string        `bson:"output,omitempty"`
	Error      string        `bson:"error,omitempty"`
	Duration   time.Duration `bson:"duration"`
	ExecutedAt time.Time     `bson:"executed_at"`
}

// CollectionName returns the MongoDB collection name for WorkflowExecution.
const WorkflowExecutionCollection = "bot_workflows"
