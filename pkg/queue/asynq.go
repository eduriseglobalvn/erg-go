// Package queue provides Asynq client and server wrappers for background job processing.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// Priority levels correspond to Asynq queue names.
const (
	PriorityCritical = "critical"
	PriorityHigh     = "high"
	PriorityDefault  = "default"
	PriorityLow      = "low"
)

// Job represents a serializable background job payload.
type Job struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// JobHandler is the function signature for processing a job.
type JobHandler func(ctx context.Context, job *Job) error

// AsynqClient wraps the Asynq client for enqueueing jobs.
type AsynqClient struct {
	client *asynq.Client
	log    *logger.Logger
	cfg    config.QueueConfig
}

// ClientOption configures the AsynqClient.
type ClientOption func(*AsynqClient)

// WithAsynqClientLogger sets the logger.
func WithAsynqClientLogger(log *logger.Logger) ClientOption {
	return func(a *AsynqClient) {
		a.log = log
	}
}

// NewAsynqClient creates a new Asynq client connected to the Redis instance.
func NewAsynqClient(cfg config.QueueConfig, opts ...ClientOption) (*AsynqClient, error) {
	c := &AsynqClient{cfg: cfg, log: logger.NoOp()}
	for _, o := range opts {
		o(c)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	c.client = client

	return c, nil
}

// Enqueue enqueues a job with the given payload and options.
// It returns the task ID on success.
func (c *AsynqClient) Enqueue(ctx context.Context, jobType string, payload interface{}, opts ...Option) (string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("queue: marshal payload: %w", err)
	}

	task := asynq.NewTask(jobType, payloadBytes)

	// Apply default options then user options.
	options := []Option{WithQueue(PriorityDefault), WithMaxRetry(c.cfg.MaxRetry)}
	options = append(options, opts...)

	var asynqOpts []asynq.Option
	for _, o := range options {
		asynqOpts = append(asynqOpts, o.asynqOpt())
	}

	info, err := c.client.EnqueueContext(ctx, task, asynqOpts...)
	if err != nil {
		return "", fmt.Errorf("queue: enqueue %s: %w", jobType, err)
	}

	return info.ID, nil
}

// EnqueueUnique enqueues a job that is unique per task ID, preventing duplicates.
func (c *AsynqClient) EnqueueUnique(ctx context.Context, jobType string, payload interface{}, ttl time.Duration, opts ...Option) (string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("queue: marshal payload: %w", err)
	}
	task := asynq.NewTask(jobType, payloadBytes)

	asynqOpts := []asynq.Option{asynq.Unique(ttl)}
	for _, o := range opts {
		asynqOpts = append(asynqOpts, o.asynqOpt())
	}

	info, err := c.client.EnqueueContext(ctx, task, asynqOpts...)
	if err != nil {
		return "", fmt.Errorf("queue: enqueue unique %s: %w", jobType, err)
	}
	return info.ID, nil
}

// Close closes the Asynq client connection.
func (c *AsynqClient) Close() error {
	return c.client.Close()
}

// AsynqServer wraps the Asynq server for processing jobs with registered handlers.
type AsynqServer struct {
	server     *asynq.Server
	log        *logger.Logger
	cfg        config.QueueConfig
	errHandler asynq.ErrorHandler
}

// ServerOption configures the AsynqServer.
type ServerOption func(*AsynqServer)

// WithAsynqServerLogger sets the logger.
func WithAsynqServerLogger(log *logger.Logger) ServerOption {
	return func(a *AsynqServer) {
		a.log = log
	}
}

// NewAsynqServer creates a new Asynq server.
func NewAsynqServer(cfg config.QueueConfig, opts ...ServerOption) (*AsynqServer, error) {
	s := &AsynqServer{cfg: cfg, log: logger.NoOp()}
	for _, o := range opts {
		o(s)
	}

	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues: map[string]int{
				PriorityCritical: 10,
				PriorityHigh:     7,
				PriorityDefault:  5,
				PriorityLow:      2,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				if cfg.RetryBackoff {
					return time.Duration(n*n) * cfg.RetryDelay
				}
				return cfg.RetryDelay
			},
			IsFailure: func(err error) bool {
				return err != nil
			},
			ErrorHandler: s.errHandler,
		},
	)
	s.server = server

	return s, nil
}

// Start starts the server with the given handler map and blocks until ctx is cancelled.
func (s *AsynqServer) Start(ctx context.Context, handlers map[string]func(ctx context.Context, t *asynq.Task) error) error {
	mux := asynq.NewServeMux()
	for jobType, handler := range handlers {
		mux.HandleFunc(jobType, handler)
	}

	s.log.Info().
		Int("concurrency", s.cfg.Concurrency).
		Msg("asynq server starting")

	if err := s.server.Start(mux); err != nil {
		return fmt.Errorf("queue: asynq server start: %w", err)
	}

	<-ctx.Done()
	s.log.Info().Msg("asynq server shutting down")
	return nil
}

// Shutdown gracefully shuts down the server with the given context deadline.
func (s *AsynqServer) Shutdown() {
	s.server.Shutdown()
}

// Option wraps Asynq options with a cleaner interface.
type Option struct {
	opt asynq.Option
}

// WithQueue sets the target queue by name.
func WithQueue(name string) Option {
	return Option{opt: asynq.Queue(name)}
}

// WithMaxRetry sets the maximum retry count.
func WithMaxRetry(n int) Option {
	return Option{opt: asynq.MaxRetry(n)}
}

// WithTimeout sets the maximum execution timeout for the task.
func WithTimeout(d time.Duration) Option {
	return Option{opt: asynq.Timeout(d)}
}

// WithDeadline sets the hard deadline for the task.
func WithDeadline(t time.Time) Option {
	return Option{opt: asynq.Deadline(t)}
}

func (o Option) asynqOpt() asynq.Option { return o.opt }

// WithErrorHandler returns a ServerOption that registers a custom error handler.
func WithErrorHandler(fn func(ctx context.Context, task *asynq.Task, err error)) ServerOption {
	h := &funcErrorHandler{fn: fn}
	return func(s *AsynqServer) {
		s.errHandler = h
	}
}

type funcErrorHandler struct {
	fn func(ctx context.Context, task *asynq.Task, err error)
}

func (h *funcErrorHandler) HandleError(ctx context.Context, task *asynq.Task, err error) {
	h.fn(ctx, task, err)
}

// ParsePayload parses the JSON payload of a task into the given struct.
func ParsePayload(t *asynq.Task, out interface{}) error {
	if err := json.Unmarshal(t.Payload(), out); err != nil {
		return fmt.Errorf("queue: parse payload: %w", err)
	}
	return nil
}
