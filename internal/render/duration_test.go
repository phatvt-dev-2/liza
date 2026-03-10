package render

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "minutes only",
			duration: 15 * time.Minute,
			want:     "15m",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 30*time.Minute,
			want:     "2h 30m",
		},
		{
			name:     "days, hours, minutes",
			duration: 25*time.Hour + 45*time.Minute,
			want:     "1d 1h 45m",
		},
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "less than a minute",
			duration: 30 * time.Second,
			want:     "30s",
		},
		{
			name:     "negative duration",
			duration: -2*time.Hour - 15*time.Minute,
			want:     "-2h 15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}
