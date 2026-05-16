package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensafe/goddddocr"
)

func TestParseFlagsRequiresImage(t *testing.T) {
	if _, err := parseFlags(nil); err == nil {
		t.Fatal("expected missing image error")
	}
}

func TestParseHSVRanges(t *testing.T) {
	ranges, err := parseHSVRanges(`[[[90,30,30],[110,255,255]]]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || ranges[0].Lower != [3]int{90, 30, 30} || ranges[0].Upper != [3]int{110, 255, 255} {
		t.Fatalf("unexpected ranges: %#v", ranges)
	}

	ranges, err = parseHSVRanges(`[[90,30,30],[110,255,255]]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || ranges[0].Lower != [3]int{90, 30, 30} || ranges[0].Upper != [3]int{110, 255, 255} {
		t.Fatalf("unexpected single range: %#v", ranges)
	}
}

func TestWriteMatrixCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matrix.csv")
	if err := writeMatrixCSV(path, []uint8{1, 2, 3, 4}, 2, 2); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "1" || rows[1][1] != "4" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestSummarize(t *testing.T) {
	report := summarize(&goddddocr.PreprocessResult{Width: 2, Height: 2}, []uint8{0, 10, 20, 30})
	if report.Width != 2 || report.Height != 2 || report.Pixels != 4 {
		t.Fatalf("unexpected dimensions: %#v", report)
	}
	if report.Min != 0 || report.Max != 30 || report.Mean != 15 {
		t.Fatalf("unexpected stats: %#v", report)
	}
	if report.SHA256 == "" {
		t.Fatal("expected hash")
	}
}
