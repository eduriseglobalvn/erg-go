// Package commands implements all bot commands for the erg-server binary.
package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"erg.ninja/internal/modules/bot/domain/entity"
)

// workflowSvc is the injected workflow engine (from bot.module.go wiring).
var workflowSvc WorkflowServiceClient

// WorkflowServiceClient is the interface for the workflow engine.
type WorkflowServiceClient interface {
	// StartWorkflow creates and starts a new workflow execution.
	StartWorkflow(ctx context.Context, workflowID, convID, userID string, steps []models.WorkflowStep) (*models.WorkflowExecution, error)
	// ExecuteNextStep runs the current step of a workflow.
	ExecuteNextStep(ctx context.Context, execID string, stepInput map[string]any) (*models.WorkflowExecution, error)
	// GetWorkflow returns a workflow execution by ID.
	GetWorkflow(ctx context.Context, execID string) (*models.WorkflowExecution, error)
	// ListWorkflows returns all workflow executions for a conversation.
	ListWorkflows(ctx context.Context, convID string, limit int64) ([]*models.WorkflowExecution, error)
	// PauseWorkflow pauses an active workflow.
	PauseWorkflow(ctx context.Context, execID string) error
	// CancelWorkflow cancels a workflow execution.
	CancelWorkflow(ctx context.Context, execID string) error
}

// SetWorkflowService injects the workflow engine service.
func SetWorkflowService(svc WorkflowServiceClient) {
	workflowSvc = svc
}

// GetWorkflowService returns the current workflow service (nil-safe).
func GetWorkflowService() WorkflowServiceClient { return workflowSvc }

// HandleWorkflowStart starts a new multi-step workflow wizard.
func HandleWorkflowStart(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if workflowSvc == nil {
		return "Workflow system is not available."
	}
	if len(args) < 1 {
		return "Usage: /workflow start <type>\nTypes: rss (add RSS feed wizard)"
	}

	wfType := strings.ToLower(strings.TrimSpace(args[0]))

	switch wfType {
	case "rss":
		return startRSSWorkflow(ctx, update)
	default:
		return fmt.Sprintf("Unknown workflow type: %s\nAvailable: rss", wfType)
	}
}

// startRSSWorkflow starts a wizard for adding an RSS feed.
func startRSSWorkflow(ctx context.Context, update *models.PlatformUpdate) string {
	steps := []models.WorkflowStep{
		{
			Name:      "ask_url",
			Type:      "input",
			OnSuccess: "validate",
			OnFail:    "ask_url",
		},
		{
			Name:      "validate",
			Type:      "crawl",
			OnSuccess: "confirm",
			OnFail:    "ask_url",
		},
		{
			Name:      "confirm",
			Type:      "notification",
			OnSuccess: "",
			OnFail:    "",
		},
	}

	exec, err := workflowSvc.StartWorkflow(ctx, "add-rss", update.ConversationID, update.UserID, steps)
	if err != nil {
		return fmt.Sprintf("Lỗi bắt đầu wizard: %v", err)
	}
	return fmt.Sprintf("Wizard bắt đầu — ID: %s\n\nStep 1/3: Nhập URL RSS feed (ví dụ: https://example.com/rss.xml)", exec.ID.Hex())
}

// HandleWorkflowStep advances a wizard step with user input.
func HandleWorkflowStep(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if workflowSvc == nil {
		return "Workflow system is not available."
	}
	if len(args) < 1 {
		return "Usage: /workflow step <exec_id> [input]"
	}

	execID := strings.TrimSpace(args[0])
	input := ""
	if len(args) > 1 {
		input = strings.Join(args[1:], " ")
	}

	exec, err := workflowSvc.ExecuteNextStep(ctx, execID, map[string]any{
		"input":   input,
		"user_id": update.UserID,
	})
	if err != nil {
		return fmt.Sprintf("Lỗi step: %v", err)
	}

	if exec.Status == models.WorkflowCompleted {
		return fmt.Sprintf("Workflow hoàn thành! ✅\nFeed đã được thêm thành công.")
	}
	if exec.Status == models.WorkflowFailed {
		return fmt.Sprintf("Workflow thất bại: %s\nGõ /workflow start rss để thử lại.", exec.Error)
	}

	return fmt.Sprintf("Step tiếp theo: %s\nGõ /workflow step %s [dữ liệu] để tiếp tục.",
		exec.CurrentStep, exec.ID.Hex())
}

// HandleWorkflowStatus shows the current status of a workflow.
func HandleWorkflowStatus(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if workflowSvc == nil {
		return "Workflow system is not available."
	}
	if len(args) < 1 {
		// List all workflows for this conversation.
		execs, err := workflowSvc.ListWorkflows(ctx, update.ConversationID, 10)
		if err != nil {
			return fmt.Sprintf("Lỗi: %v", err)
		}
		if len(execs) == 0 {
			return "Không có workflow nào đang chạy."
		}
		var lines []string
		lines = append(lines, "Workflows đang chạy:")
		for _, exec := range execs {
			lines = append(lines, fmt.Sprintf("- %s [%s] step: %s",
				exec.WorkflowID, exec.Status, exec.CurrentStep))
		}
		return strings.Join(lines, "\n")
	}

	execID := strings.TrimSpace(args[0])
	exec, err := workflowSvc.GetWorkflow(ctx, execID)
	if err != nil {
		return fmt.Sprintf("Lỗi: %v", err)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Workflow: %s", exec.WorkflowID))
	lines = append(lines, fmt.Sprintf("Status: %s", exec.Status))
	lines = append(lines, fmt.Sprintf("Current step: %s", exec.CurrentStep))
	lines = append(lines, fmt.Sprintf("Started: %s", exec.StartedAt.Format(time.RFC822)))
	if exec.CompletedAt != nil {
		lines = append(lines, fmt.Sprintf("Completed: %s", exec.CompletedAt.Format(time.RFC822)))
	}
	if exec.Error != "" {
		lines = append(lines, fmt.Sprintf("Error: %s", exec.Error))
	}
	lines = append(lines, fmt.Sprintf("\nRetry count: %d/%d", exec.RetryCount, exec.MaxRetries))
	return strings.Join(lines, "\n")
}

// HandleWorkflowCancel cancels or pauses a workflow.
func HandleWorkflowCancel(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if workflowSvc == nil {
		return "Workflow system is not available."
	}
	if len(args) < 1 {
		return "Usage: /workflow cancel <exec_id>"
	}
	execID := strings.TrimSpace(args[0])
	if err := workflowSvc.CancelWorkflow(ctx, execID); err != nil {
		return fmt.Sprintf("Lỗi hủy workflow: %v", err)
	}
	return fmt.Sprintf("Workflow %s đã bị hủy.", execID)
}
