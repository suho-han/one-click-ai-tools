package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/schedule"
)

func TestSessionRefreshScheduleConfigNormalizesFixedIntervals(t *testing.T) {
	viper.Reset()
	viper.Set("session_refresh_enabled", true)
	viper.Set("session_refresh_interval", "6h")
	viper.Set("session_refresh_hour", 25)

	enabled, interval, hour := sessionRefreshScheduleConfig()
	if !enabled {
		t.Fatal("expected enabled=true")
	}
	if interval != schedule.SixHourInterval {
		t.Fatalf("interval = %q, want %q", interval, schedule.SixHourInterval)
	}
	if hour != 9 {
		t.Fatalf("hour = %d, want default 9", hour)
	}
}

func TestSessionRefreshScheduleConfigFallsBackToDaily(t *testing.T) {
	viper.Reset()
	viper.Set("session_refresh_interval", "monthly")
	viper.Set("session_refresh_hour", 7)

	_, interval, hour := sessionRefreshScheduleConfig()
	if interval != schedule.DailyInterval {
		t.Fatalf("interval = %q, want %q", interval, schedule.DailyInterval)
	}
	if hour != 7 {
		t.Fatalf("hour = %d, want 7", hour)
	}
}
