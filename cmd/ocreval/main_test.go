package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfigRequiresInput(t *testing.T) {
	if err := validateConfig(evalConfig{}); err == nil {
		t.Fatal("expected missing input error")
	}
}

func TestScanDirectorySamples(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "3n3d.png"), []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "abcd.jpg"), []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}

	samples, err := scanDirectorySamples(dir, true, "filename")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 {
		t.Fatalf("sample count = %d, want 2", len(samples))
	}
	if samples[0].Expected != "3n3d" || samples[1].Expected != "abcd" {
		t.Fatalf("unexpected expected values: %#v", samples)
	}

	samples, err = scanDirectorySamples(dir, false, "none")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("non-recursive sample count = %d, want 1", len(samples))
	}
	if samples[0].Expected != "" {
		t.Fatalf("expected should be empty: %#v", samples[0])
	}
}

func TestSummarize(t *testing.T) {
	summary := summarize([]evalResult{
		{Name: "matched", Labeled: true, Matched: true},
		{Name: "mismatched", Labeled: true},
		{Name: "unlabeled"},
		{Name: "errored", Error: "boom"},
	})
	if summary.Total != 4 || summary.Labeled != 2 || summary.Unlabeled != 1 || summary.Matched != 1 || summary.Mismatched != 1 || summary.Errors != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.Accuracy != 0.5 {
		t.Fatalf("unexpected accuracy: %f", summary.Accuracy)
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" red, blue ,,green ")
	want := []string{"red", "blue", "green"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("item %d = %q, want %q", idx, got[idx], want[idx])
		}
	}
}

func TestMarkdownSummaryEscapesPipes(t *testing.T) {
	md := markdownSummary(evalSummary{
		Total: 1,
		Results: []evalResult{
			{Name: "a|b", Expected: "x|y", Result: "x|y", Matched: true},
		},
	})
	if !strings.Contains(md, "a\\|b") || !strings.Contains(md, "x\\|y") {
		t.Fatalf("markdown did not escape pipes:\n%s", md)
	}
}
