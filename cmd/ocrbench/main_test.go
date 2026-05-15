package main

import (
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	values := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}

	if got := percentile(values, 0.50); got != 20*time.Millisecond {
		t.Fatalf("p50 = %v", got)
	}
	if got := percentile(values, 0.95); got != 40*time.Millisecond {
		t.Fatalf("p95 = %v", got)
	}
	if got := percentile(values, 1); got != 40*time.Millisecond {
		t.Fatalf("p100 = %v", got)
	}
}

func TestSummarizeBench(t *testing.T) {
	results := []benchResult{
		{Duration: 10 * time.Millisecond, Text: "3n3d"},
		{Duration: 20 * time.Millisecond, Error: "boom"},
		{Duration: 30 * time.Millisecond, Mismatch: true, Error: "mismatch"},
	}

	summary := summarizeBench(benchConfig{
		BaseURL:     "http://127.0.0.1:8088",
		ImagePath:   "samples/yzm1.png",
		Requests:    3,
		Concurrency: 2,
	}, 100*time.Millisecond, results)

	if summary.Success != 1 {
		t.Fatalf("success = %d", summary.Success)
	}
	if summary.Errors != 2 {
		t.Fatalf("errors = %d", summary.Errors)
	}
	if summary.Mismatches != 1 {
		t.Fatalf("mismatches = %d", summary.Mismatches)
	}
	if summary.QPS != 30 {
		t.Fatalf("qps = %f", summary.QPS)
	}
	if summary.P95LatencyMS != 30 {
		t.Fatalf("p95 = %f", summary.P95LatencyMS)
	}
	if summary.FirstError != "boom" {
		t.Fatalf("first error = %q", summary.FirstError)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := map[string]string{
		"http://127.0.0.1:8088/ocr":      "http://127.0.0.1:8088",
		"http://127.0.0.1:8088/ocr/file": "http://127.0.0.1:8088",
		"http://127.0.0.1:8088/ready":    "http://127.0.0.1:8088",
		"http://127.0.0.1:8088/api":      "http://127.0.0.1:8088/api",
	}
	for input, want := range tests {
		if got := normalizeBaseURL(input); got != want {
			t.Fatalf("normalizeBaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}
