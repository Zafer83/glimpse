package ai

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"HTTP 429", &HTTPError{StatusCode: 429, Body: "rate limited"}, true},
		{"HTTP 500", &HTTPError{StatusCode: 500, Body: "server error"}, true},
		{"HTTP 502", &HTTPError{StatusCode: 502, Body: "bad gateway"}, true},
		{"HTTP 503", &HTTPError{StatusCode: 503, Body: "unavailable"}, true},
		{"HTTP 504", &HTTPError{StatusCode: 504, Body: "timeout"}, true},
		{"HTTP 400 not retryable", &HTTPError{StatusCode: 400, Body: "bad request"}, false},
		{"HTTP 401 not retryable", &HTTPError{StatusCode: 401, Body: "unauthorized"}, false},
		{"HTTP 404 not retryable", &HTTPError{StatusCode: 404, Body: "not found"}, false},
		{"connection refused", fmt.Errorf("dial tcp: connection refused"), true},
		{"connection reset", fmt.Errorf("connection reset by peer"), true},
		{"EOF", fmt.Errorf("unexpected EOF"), true},
		{"random error", fmt.Errorf("something else"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// timeoutError implements net.Error with Timeout() = true.
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

var _ net.Error = (*timeoutError)(nil)

func TestIsRetryable_NetTimeout(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &timeoutError{})
	if !isRetryable(err) {
		t.Error("net.Error with Timeout() should be retryable")
	}
}

func TestBackoffDelay(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
	}

	// Attempt 0 should be around BaseDelay (±25% jitter).
	d0 := backoffDelay(cfg, 0)
	if d0 < 75*time.Millisecond || d0 > 125*time.Millisecond {
		t.Errorf("attempt 0 delay %v outside expected range [75ms, 125ms]", d0)
	}

	// Attempt 1 should be around 2*BaseDelay.
	d1 := backoffDelay(cfg, 1)
	if d1 < 150*time.Millisecond || d1 > 250*time.Millisecond {
		t.Errorf("attempt 1 delay %v outside expected range [150ms, 250ms]", d1)
	}

	// High attempt should be capped at MaxDelay (±jitter).
	d10 := backoffDelay(cfg, 10)
	if d10 > cfg.MaxDelay+cfg.MaxDelay/4 {
		t.Errorf("attempt 10 delay %v exceeds max+jitter %v", d10, cfg.MaxDelay+cfg.MaxDelay/4)
	}
}

func TestHTTPError(t *testing.T) {
	err := &HTTPError{StatusCode: 503, Body: "service unavailable"}
	if err.Error() != "HTTP 503: service unavailable" {
		t.Errorf("unexpected Error() = %q", err.Error())
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Error("should be unwrappable as *HTTPError")
	}
}
