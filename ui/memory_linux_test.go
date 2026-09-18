package main

import "testing"

func TestParseMemAvailable(t *testing.T) {
	meminfo := []byte("MemTotal:       65536 kB\nMemFree:         1024 kB\nMemAvailable:    2048 kB\n")
	if got := parseMemAvailable(meminfo); got != 2048*1024 {
		t.Errorf("got %d", got)
	}
	if got := parseMemAvailable([]byte("nothing useful")); got != 0 {
		t.Errorf("got %d, want 0 for unknown", got)
	}
}
