package proxy

import (
	"errors"
	"net/http"
	"testing"

	"api-gateway/internal/domain"
)

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		attempt     int
		maxAttempts int
		status      int
		err         error
		want        bool
	}{
		{name: "retry transport error", method: http.MethodGet, attempt: 1, maxAttempts: 2, err: errors.New("boom"), want: true},
		{name: "retry 503", method: http.MethodGet, attempt: 1, maxAttempts: 2, status: http.StatusServiceUnavailable, want: true},
		{name: "do not retry on last attempt", method: http.MethodGet, attempt: 2, maxAttempts: 2, err: errors.New("boom"), want: false},
		{name: "do not retry non idempotent", method: http.MethodPost, attempt: 1, maxAttempts: 2, err: errors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.status != 0 {
				resp = &http.Response{StatusCode: tt.status}
			}
			if got := shouldRetry(tt.method, tt.attempt, tt.maxAttempts, resp, tt.err); got != tt.want {
				t.Fatalf("shouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaxRetryAttempts(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	cfg := &domain.RouteRetry{Enabled: true, MaxAttempts: 3, BackoffMS: 10}
	if got := maxRetryAttempts(req, cfg); got != 3 {
		t.Fatalf("maxRetryAttempts() = %d, want 3", got)
	}
}
