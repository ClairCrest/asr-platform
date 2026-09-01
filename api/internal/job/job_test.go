package job

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{"pending to queued", StatusPending, StatusQueued, true},
		{"pending to processing skips queued", StatusPending, StatusProcessing, false},
		{"queued to processing", StatusQueued, StatusProcessing, true},
		{"queued to cancelled", StatusQueued, StatusCancelled, true},
		{"queued to succeeded", StatusQueued, StatusSucceeded, false},
		{"processing to succeeded", StatusProcessing, StatusSucceeded, true},
		{"processing to failed", StatusProcessing, StatusFailed, true},
		{"processing to retrying", StatusProcessing, StatusRetrying, true},
		{"processing to cancelled", StatusProcessing, StatusCancelled, true},
		{"processing to queued direct", StatusProcessing, StatusQueued, false},
		{"retrying to queued", StatusRetrying, StatusQueued, true},
		{"retrying to processing skips queued", StatusRetrying, StatusProcessing, false},
		{"succeeded is terminal", StatusSucceeded, StatusQueued, false},
		{"failed has no automatic transition", StatusFailed, StatusQueued, false},
		{"cancelled is terminal", StatusCancelled, StatusQueued, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestCanCancel(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusPending, false},
		{StatusQueued, true},
		{StatusProcessing, true},
		{StatusRetrying, false},
		{StatusSucceeded, false},
		{StatusFailed, false},
		{StatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := CanCancel(tt.status); got != tt.want {
				t.Errorf("CanCancel(%s) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestCanRetry(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusPending, false},
		{StatusQueued, false},
		{StatusProcessing, false},
		{StatusRetrying, false},
		{StatusSucceeded, false},
		{StatusFailed, true},
		{StatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := CanRetry(tt.status); got != tt.want {
				t.Errorf("CanRetry(%s) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
