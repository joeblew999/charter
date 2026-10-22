package bench

import (
	"testing"
	"time"
)

func TestParseCPUTime(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"0:01.23":    1230 * time.Millisecond, // macOS: m:ss.cc
		"00:00:05":   5 * time.Second,         // Linux procps: hh:mm:ss
		"1-02:03:04": 26*time.Hour + 3*time.Minute + 4*time.Second,
		"12:34.50":   12*time.Minute + 34500*time.Millisecond,
	} {
		if got := parseCPUTime(in); got != want {
			t.Errorf("parseCPUTime(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMedian(t *testing.T) {
	if m := median([]float64{3, 1, 2}); m != 2 {
		t.Errorf("odd: %v", m)
	}
	if m := median([]float64{4, 1, 3, 2}); m != 2.5 {
		t.Errorf("even: %v", m)
	}
}
