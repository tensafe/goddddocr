package main

import (
	"encoding/csv"
	"image"
	"image/color"
	"image/png"
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

func TestParseFlagsRequiresReferenceForDiffPNG(t *testing.T) {
	_, err := parseFlags([]string{"-image", "sample.png", "-diff-png", "diff.png"})
	if err == nil {
		t.Fatal("expected diff-png reference error")
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

func TestReadMatrixCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matrix.csv")
	if err := os.WriteFile(path, []byte("1,2\n3,4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pixels, width, height, err := readMatrixCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if width != 2 || height != 2 {
		t.Fatalf("dimensions = %dx%d", width, height)
	}
	if len(pixels) != 4 || pixels[0] != 1 || pixels[3] != 4 {
		t.Fatalf("unexpected pixels: %#v", pixels)
	}
}

func TestReadGrayPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gray.png")
	img := image.NewGray(image.Rect(0, 0, 2, 1))
	img.SetGray(0, 0, color.Gray{Y: 7})
	img.SetGray(1, 0, color.Gray{Y: 9})
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	pixels, width, height, err := readGrayPNG(path)
	if err != nil {
		t.Fatal(err)
	}
	if width != 2 || height != 1 || len(pixels) != 2 || pixels[0] != 7 || pixels[1] != 9 {
		t.Fatalf("unexpected png read: %dx%d %#v", width, height, pixels)
	}
}

func TestComparePixels(t *testing.T) {
	report, err := comparePixels([]uint8{10, 20, 30}, []uint8{10, 18, 33})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExactMatch {
		t.Fatal("expected non-exact match")
	}
	if report.DifferentPixels != 2 || report.MaxAbsDiff != 3 {
		t.Fatalf("unexpected diff report: %#v", report)
	}
	if report.MeanAbsDiff != 1.667 || report.RMSE != 2.082 {
		t.Fatalf("unexpected diff stats: %#v", report)
	}
	if report.ReferenceSHA256 == "" {
		t.Fatal("expected reference hash")
	}
}

func TestCompareReferenceRejectsBothReferenceTypes(t *testing.T) {
	_, err := compareReference(prepConfig{ComparePNGPath: "a.png", CompareCSVPath: "b.csv"}, []uint8{1}, 1, 1)
	if err == nil {
		t.Fatal("expected mutually-exclusive reference error")
	}
}

func TestWriteDiffPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diff.png")
	err := writeDiffPNG(path, []uint8{10, 20, 30}, []uint8{10, 25, 20}, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 3 || img.Bounds().Dy() != 1 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
	first := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
	second := color.RGBAModel.Convert(img.At(1, 0)).(color.RGBA)
	third := color.RGBAModel.Convert(img.At(2, 0)).(color.RGBA)
	if first.R != 0 || first.G != 0 || first.B != 0 || first.A != 255 {
		t.Fatalf("expected black exact pixel, got %#v", first)
	}
	if second.R == 0 || second.G != 0 || second.B != 0 {
		t.Fatalf("expected red darker pixel, got %#v", second)
	}
	if third.R != 0 || third.G != 0 || third.B == 0 {
		t.Fatalf("expected blue brighter pixel, got %#v", third)
	}
}

func TestWriteDiffPNGRejectsMismatchedPixels(t *testing.T) {
	err := writeDiffPNG(filepath.Join(t.TempDir(), "diff.png"), []uint8{1, 2}, []uint8{1}, 2, 1)
	if err == nil {
		t.Fatal("expected mismatched pixel error")
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
