/*
Copyright 2026 Zafer Kılıçaslan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ai

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

// RetryConfig controls retry behaviour for transient AI provider errors.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryConfig is used for all AI provider calls.
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   2 * time.Second,
	MaxDelay:    30 * time.Second,
}

// HTTPError wraps a non-2xx HTTP response so the retry logic can inspect the
// status code without string parsing.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// isRetryable returns true for transient errors that are worth retrying:
// network timeouts, connection resets, and server-side HTTP errors.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// HTTP status codes that indicate a transient server problem.
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case 429, 500, 502, 503, 504:
			return true
		}
		return false
	}

	// Network-level transient errors.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "EOF")
}

// backoffDelay computes the sleep duration for the given attempt using
// exponential backoff with ±25 % jitter.
func backoffDelay(cfg RetryConfig, attempt int) time.Duration {
	delay := cfg.BaseDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
			break
		}
	}
	// Jitter: ±25 %
	jitter := time.Duration(float64(delay) * (0.5*rand.Float64() - 0.25))
	return delay + jitter
}
