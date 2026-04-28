// Package http provides HTTP client with retry, circuit breaker, and structured logging.
package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"erg.ninja/pkg/logger"
)

// Client wraps http.Client with retry, circuit breaker, and logging.
type Client struct {
	client  *http.Client
	baseURL string
	log     *logger.Logger

	// Circuit breaker state.
	mu              sync.Mutex
	circuitState    int32 // 0=closed, 1=half-open, 2=open
	failureCount    int32
	successCount    int32
	lastFailure     time.Time
	openTimeout     time.Duration
	threshold       int32 // failures before opening circuit
	halfOpenRetries int32

	// Retry config.
	maxRetries  int
	retryCodes  []int // HTTP status codes that trigger retry
	retryDelays []time.Duration
}

// ClientOption configures an HTTP client.
type ClientOption func(*Client)

// WithBaseURL sets the base URL prepended to all request paths.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithLogger sets the logger for the HTTP client.
func WithLogger(log *logger.Logger) ClientOption {
	return func(c *Client) {
		c.log = log
	}
}

// WithTransport configures the underlying http.Transport.
func WithTransport(transport *http.Transport) ClientOption {
	return func(c *Client) {
		c.client.Transport = transport
	}
}

// WithTLSConfig configures TLS settings.
func WithTLSConfig(tlsCfg *tls.Config) ClientOption {
	return func(c *Client) {
		c.client.Transport = &http.Transport{
			TLSClientConfig: tlsCfg,
			Proxy:           http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
}

// NewClient creates a new HTTP client with production defaults:
// 5s default timeout, 3 retries with exponential backoff, circuit breaker.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		log:         logger.NoOp(),
		maxRetries:  3,
		threshold:   5,
		openTimeout: 30 * time.Second,
		retryCodes:  []int{500, 502, 503, 504, 408, 429},
		retryDelays: []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
		},
	}

	// Apply options first, then set defaults.
	for _, o := range opts {
		o(c)
	}

	// Set default HTTP client if not configured.
	if c.client == nil {
		c.client = &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
	}

	return c
}

// circuitState constants.
const (
	circuitClosed   int32 = 0
	circuitHalfOpen int32 = 1
	circuitOpen     int32 = 2
)

// isRetryable returns true if the status code should trigger a retry.
func isRetryable(code int, retryCodes []int) bool {
	for _, c := range retryCodes {
		if c == code {
			return true
		}
	}
	return false
}

// Do performs an HTTP request with retry and circuit breaker protection.
// It retries on 5xx errors (not 4xx) with exponential backoff and jitter.
func (c *Client) Do(ctx context.Context, method, url string, body []byte, headers http.Header) (*http.Response, error) {
	// Check circuit breaker.
	if !c.allowRequest() {
		return nil, fmt.Errorf("http: circuit breaker open, retry after %v", c.openTimeout)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelays[min(attempt-1, len(c.retryDelays)-1)]
			jitter := time.Duration(rand.Int63n(int64(delay)/2+1)) * time.Millisecond * 100
			wait := delay + jitter

			c.log.DebugContext(ctx).
				Int("attempt", attempt).
				Dur("retry_delay", wait).
				Str("url", url).
				Msg("http: retrying request")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("http: new request: %w", err)
		}

		// Apply custom headers.
		if headers != nil {
			for key, values := range headers {
				for _, v := range values {
					req.Header.Add(key, v)
				}
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http: do %s %s: %w", method, url, err)
			c.recordFailure()
			continue
		}

		// Retry only on server errors (5xx), not client errors (4xx).
		if !isRetryable(resp.StatusCode, c.retryCodes) {
			c.recordSuccess()
			return resp, nil
		}

		// Consume the body to allow connection reuse.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		lastErr = fmt.Errorf("http: %s %s: server error %d", method, url, resp.StatusCode)
		c.recordFailure()
	}

	return nil, fmt.Errorf("http: %s %s: max retries exceeded: %w", method, url, lastErr)
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, url string, headers http.Header) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, url, nil, headers)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, url string, body []byte, headers http.Header) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, url, body, headers)
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, url string, body []byte, headers http.Header) (*http.Response, error) {
	return c.Do(ctx, http.MethodPut, url, body, headers)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, url string, headers http.Header) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, url, nil, headers)
}

// allowRequest checks the circuit breaker state before allowing a request.
func (c *Client) allowRequest() bool {
	state := atomic.LoadInt32(&c.circuitState)

	switch state {
	case circuitClosed:
		return true
	case circuitOpen:
		c.mu.Lock()
		defer c.mu.Unlock()
		if time.Since(c.lastFailure) > c.openTimeout {
			atomic.StoreInt32(&c.circuitState, circuitHalfOpen)
			return true
		}
		return false
	case circuitHalfOpen:
		retries := atomic.LoadInt32(&c.halfOpenRetries)
		if retries < 3 {
			atomic.AddInt32(&c.halfOpenRetries, 1)
			return true
		}
		return false
	}
	return true
}

// recordFailure increments the failure counter and opens the circuit if threshold is reached.
func (c *Client) recordFailure() {
	failures := atomic.AddInt32(&c.failureCount, 1)
	c.lastFailure = time.Now()

	if failures >= c.threshold {
		atomic.StoreInt32(&c.circuitState, circuitOpen)
	}
}

// recordSuccess resets the circuit to closed state.
func (c *Client) recordSuccess() {
	atomic.StoreInt32(&c.failureCount, 0)
	atomic.StoreInt32(&c.circuitState, circuitClosed)
}

// CircuitState returns the current circuit breaker state as a string.
func (c *Client) CircuitState() string {
	switch atomic.LoadInt32(&c.circuitState) {
	case circuitClosed:
		return "closed"
	case circuitHalfOpen:
		return "half-open"
	case circuitOpen:
		return "open"
	}
	return "unknown"
}
