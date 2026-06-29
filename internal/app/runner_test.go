package app

import (
	"testing"
	"time"
)

func TestStartDelaySpreadsUsersAcrossRamp(t *testing.T) {
	cfg := Config{Users: 4, Ramp: 12 * time.Second}

	tests := []struct {
		userID int
		want   time.Duration
	}{
		{userID: 1, want: 0},
		{userID: 2, want: 3 * time.Second},
		{userID: 3, want: 6 * time.Second},
		{userID: 4, want: 9 * time.Second},
	}

	for _, tt := range tests {
		got := startDelay(cfg, tt.userID)
		if got != tt.want {
			t.Fatalf("startDelay(user=%d) = %s, want %s", tt.userID, got, tt.want)
		}
	}
}

func TestStartDelayDisabledForSingleUserOrZeroRamp(t *testing.T) {
	if got := startDelay(Config{Users: 1, Ramp: 10 * time.Second}, 1); got != 0 {
		t.Fatalf("single user delay = %s, want 0", got)
	}
	if got := startDelay(Config{Users: 10, Ramp: 0}, 10); got != 0 {
		t.Fatalf("zero ramp delay = %s, want 0", got)
	}
}
