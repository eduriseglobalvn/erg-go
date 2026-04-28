package interceptors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ergerr "erg.ninja/pkg/errors"
	"github.com/rs/zerolog"
)

// dummyLogger returns a disabled *zerolog.Logger so tests don't spam stdout.
func dummyLogger() *zerolog.Logger {
	l := zerolog.New(nil)
	return &l
}

// ─── ErrorInterceptor ────────────────────────────────────────────────────────

func TestErrorInterceptorPanic(t *testing.T) {
	logger := dummyLogger()
	middleware := ErrorInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("panic recovery: got status %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"code"`) {
		t.Errorf("panic response: got body %q, want JSON with code field", body)
	}
}

func TestErrorInterceptorPanicNil(t *testing.T) {
	logger := dummyLogger()
	middleware := ErrorInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// panic(nil) creates a non-nil interface value holding nil. recover() returns
	// that non-nil interface → handlePanic is called → 500.
	middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("nil panic: got status %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestErrorInterceptorNoError(t *testing.T) {
	logger := dummyLogger()
	middleware := ErrorInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("no error: got status %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"ok":true`) {
		t.Errorf("no error: got body %q, want %q", body, `{"ok":true}`)
	}
}

func TestErrorInterceptorSetsContentType(t *testing.T) {
	logger := dummyLogger()
	middleware := ErrorInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("panic for content-type check")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("panic response Content-Type: got %q, want application/json", ct)
	}
}

func TestErrorInterceptorSetsRequestIDHeader(t *testing.T) {
	logger := dummyLogger()
	middleware := ErrorInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("trigger error for request ID header")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	xReqID := rec.Header().Get("X-Request-ID")
	if xReqID == "" {
		t.Errorf("X-Request-ID header: got empty, want non-empty")
	}
}

// ─── WrapHandler ─────────────────────────────────────────────────────────────

func TestWrapHandlerReturnsError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return ergerr.ErrCrawlerNotFound.ToError("feed not found")
	}

	req := httptest.NewRequest(http.MethodGet, "/feeds/123", nil)
	rec := httptest.NewRecorder()

	wrapped := WrapHandler(handler)
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("handler error: got status %d, want %d", rec.Code, http.StatusNotFound)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"code":"CRAWLER_NOT_FOUND"`) {
		t.Errorf("handler error body: got %q", body)
	}
	if !strings.Contains(body, `"message":"feed not found"`) {
		t.Errorf("handler error message missing: got %q", body)
	}
}

func TestWrapHandlerNoError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"42"}`))
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	rec := httptest.NewRecorder()

	wrapped := WrapHandler(handler)
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("no error: got status %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestWrapHandlerPanic(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		panic("handler panic inside WrapHandler")
	}

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	wrapped := WrapHandler(handler)
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("panic in WrapHandler: got status %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"code"`) {
		t.Errorf("panic response body: got %q", body)
	}
}

func TestWrapHandlerBadRequest(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return ergerr.ErrBadRequest.ToError("invalid input")
	}

	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	rec := httptest.NewRecorder()

	wrapped := WrapHandler(handler)
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad request: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"code":"BAD_REQUEST"`) {
		t.Errorf("bad request body: got %q", body)
	}
}

func TestWrapHandlerContentType(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return ergerr.ErrInternal.ToError("boom")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	wrapped := WrapHandler(handler)
	wrapped.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("WrapHandler Content-Type: got %q, want application/json", ct)
	}
}

func TestWrapHandlerPlainErrorFallsBackToInternal(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error {
		return ergerr.Errorf(ergerr.ErrServiceUnavailable, "service down")
	}

	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	rec := httptest.NewRecorder()

	wrapped := WrapHandler(handler)
	wrapped.ServeHTTP(rec, req)

	// Plain errors (non-*E) that become nil FromError fall back to 500.
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("plain error: got status %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// ─── AddTraceID ──────────────────────────────────────────────────────────────

func TestAddTraceIDFromHeader(t *testing.T) {
	middleware := AddTraceID()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID, _ := r.Context().Value(TraceIDKey).(string)
		w.Header().Set("X-Trace-ID", traceID)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Trace-ID", "abc-123-trace")

	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	if rec.Header().Get("X-Trace-ID") != "abc-123-trace" {
		t.Errorf("AddTraceID from header: got %q, want %q", rec.Header().Get("X-Trace-ID"), "abc-123-trace")
	}
}

func TestAddTraceIDNoHeader(t *testing.T) {
	middleware := AddTraceID()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID, _ := r.Context().Value(TraceIDKey).(string)
		w.Header().Set("X-Trace-ID", traceID)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	// No X-Trace-ID header; context value is empty string.
	if rec.Header().Get("X-Trace-ID") != "" {
		t.Errorf("AddTraceID no header: got %q, want empty string", rec.Header().Get("X-Trace-ID"))
	}
}

// ─── statusCaptureWriter ────────────────────────────────────────────────────

func TestStatusCaptureWriterDefaultsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &statusCaptureWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	if rw.status != http.StatusNotFound {
		t.Errorf("statusCaptureWriter: captured status %d, want %d", rw.status, http.StatusNotFound)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("statusCaptureWriter: wrapped ResponseWriter got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestStatusCaptureWriterPreservesOtherHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &statusCaptureWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.Header().Set("Content-Type", "text/plain")
	rw.WriteHeader(http.StatusAccepted)

	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("Content-Type preserved: got %q, want %q", ct, "text/plain")
	}
}

// ─── writeError ──────────────────────────────────────────────────────────────

func TestWriteErrorSetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := ergerr.NewResponse(ergerr.ErrBadRequest, "bad")
	writeError(rec, resp, http.StatusBadRequest)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json; charset=utf-8")
	}
}

func TestWriteErrorSetsRequestIDHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := ergerr.NewResponseWithRequestID(ergerr.ErrNotFound, "not found", "req-xyz")
	writeError(rec, resp, http.StatusNotFound)

	if got := rec.Header().Get("X-Request-ID"); got != "req-xyz" {
		t.Errorf("X-Request-ID: got %q, want %q", got, "req-xyz")
	}
}

func TestWriteErrorSetsTraceIDHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := ergerr.NewResponse(ergerr.ErrBadRequest, "bad").WithTraceID("trace-abc")
	writeError(rec, resp, http.StatusBadRequest)

	if got := rec.Header().Get("X-Trace-ID"); got != "trace-abc" {
		t.Errorf("X-Trace-ID: got %q, want %q", got, "trace-abc")
	}
}

func TestWriteErrorSetsRetryAfterHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := ergerr.NewResponse(ergerr.ErrServiceUnavailable, "unavailable").
		WithRetryAfter(300)
	writeError(rec, resp, http.StatusServiceUnavailable)

	if got := rec.Header().Get("Retry-After"); got != "300" {
		t.Errorf("Retry-After: got %q, want %q", got, "300")
	}
}

func TestWriteErrorBodyContainsCodeAndMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := ergerr.NewResponse(ergerr.ErrAlreadyExists, "item already exists")
	writeError(rec, resp, http.StatusConflict)

	body := rec.Body.String()
	if !strings.Contains(body, `"code":"ALREADY_EXISTS"`) {
		t.Errorf("body code: got %q", body)
	}
	if !strings.Contains(body, `"message":"item already exists"`) {
		t.Errorf("body message: got %q", body)
	}
}

// ─── ergerr.Code → HTTP status ───────────────────────────────────────────────

func TestErrorResponseHTTPStatus(t *testing.T) {
	tests := []struct {
		code       ergerr.Code
		wantStatus int
	}{
		{ergerr.ErrBadRequest, http.StatusBadRequest},
		{ergerr.ErrNotFound, http.StatusNotFound},
		{ergerr.ErrInternal, http.StatusInternalServerError},
		{ergerr.ErrServiceUnavailable, http.StatusServiceUnavailable},
		{ergerr.ErrUnauthenticated, http.StatusUnauthorized},
		{ergerr.ErrForbidden, http.StatusForbidden},
		{ergerr.ErrAlreadyExists, http.StatusConflict},
		{ergerr.ErrCrawlerRateLimited, http.StatusTooManyRequests},
		{ergerr.ErrUnprocessableEntity, http.StatusUnprocessableEntity},
	}

	for _, tc := range tests {
		got := tc.code.ToHTTPStatus()
		if got != tc.wantStatus {
			t.Errorf("%s: got %d, want %d", tc.code, got, tc.wantStatus)
		}
	}
}

// ─── NewResponse helpers ─────────────────────────────────────────────────────

func TestNewResponse(t *testing.T) {
	resp := ergerr.NewResponse(ergerr.ErrBadRequest, "invalid input")

	if resp.Code != ergerr.ErrBadRequest {
		t.Errorf("Code: got %s, want %s", resp.Code, ergerr.ErrBadRequest)
	}
	if resp.Message != "invalid input" {
		t.Errorf("Message: got %q, want %q", resp.Message, "invalid input")
	}
}

func TestNewResponseWithRequestID(t *testing.T) {
	resp := ergerr.NewResponseWithRequestID(ergerr.ErrNotFound, "not found", "req-abc")

	if resp.Code != ergerr.ErrNotFound {
		t.Errorf("Code: got %s, want %s", resp.Code, ergerr.ErrNotFound)
	}
	if resp.RequestID != "req-abc" {
		t.Errorf("RequestID: got %q, want %q", resp.RequestID, "req-abc")
	}
}

// ─── FromError ────────────────────────────────────────────────────────────────

func TestFromErrorNil(t *testing.T) {
	resp := ergerr.FromError(nil)
	if resp != nil {
		t.Errorf("FromError(nil): got %v, want nil", resp)
	}
}

func TestFromErrorStructured(t *testing.T) {
	err := ergerr.ErrCrawlerRateLimited.ToError("rate limit exceeded")
	resp := ergerr.FromError(err)

	if resp == nil {
		t.Fatalf("FromError(structured): got nil, want non-nil")
	}
	if resp.Code != ergerr.ErrCrawlerRateLimited {
		t.Errorf("Code: got %s, want %s", resp.Code, ergerr.ErrCrawlerRateLimited)
	}
	if resp.Message != "rate limit exceeded" {
		t.Errorf("Message: got %q, want %q", resp.Message, "rate limit exceeded")
	}
}

func TestFromErrorPlainError(t *testing.T) {
	err := ergerr.Errorf(ergerr.ErrInternal, "something went wrong")
	resp := ergerr.FromError(err)

	if resp == nil {
		t.Fatalf("FromError(plain): got nil, want non-nil")
	}
	if resp.Code != ergerr.ErrInternal {
		t.Errorf("plain Code: got %s, want %s", resp.Code, ergerr.ErrInternal)
	}
	if resp.Message == "" {
		t.Errorf("plain Message: got empty, want non-empty")
	}
}

// ─── ErrorResponse.With* ─────────────────────────────────────────────────────

func TestErrorResponseWithTraceID(t *testing.T) {
	resp := ergerr.NewResponse(ergerr.ErrBadRequest, "bad").
		WithTraceID("trace-xyz")

	if resp.TraceID != "trace-xyz" {
		t.Errorf("WithTraceID: got %q, want %q", resp.TraceID, "trace-xyz")
	}
	// Original is unchanged.
	orig := ergerr.NewResponse(ergerr.ErrBadRequest, "bad")
	if orig.TraceID != "" {
		t.Errorf("original WithTraceID: got %q, want empty", orig.TraceID)
	}
}

func TestErrorResponseWithRetryAfter(t *testing.T) {
	resp := ergerr.NewResponse(ergerr.ErrServiceUnavailable, "unavailable").
		WithRetryAfter(60)

	if resp.RetryAfter != 60 {
		t.Errorf("WithRetryAfter: got %d, want %d", resp.RetryAfter, 60)
	}
}

func TestErrorResponseWithDetails(t *testing.T) {
	resp := ergerr.NewResponse(ergerr.ErrBadRequest, "bad").
		WithDetails("field X is required")

	if resp.Details != "field X is required" {
		t.Errorf("WithDetails: got %q, want %q", resp.Details, "field X is required")
	}
}
