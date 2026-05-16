package goddddocr

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"testing"
)

func TestSlideComparisonImagesFindsLargestDiff(t *testing.T) {
	target, background := syntheticSlideImages()

	result, err := SlideComparisonImages(target, background)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Target) != 2 {
		t.Fatalf("target = %#v", result.Target)
	}
	if result.TargetX != 51 || result.TargetY != 32 {
		t.Fatalf("target = %#v, want [51 32]", result)
	}
}

func TestSlideComparisonImagesNoDiff(t *testing.T) {
	_, background := syntheticSlideImages()

	result, err := SlideComparisonImages(background, background)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Target) != 2 || result.Target[0] != 0 || result.Target[1] != 0 {
		t.Fatalf("target = %#v, want [0 0]", result.Target)
	}
}

func TestSlideComparisonImagesRejectsDimensionMismatch(t *testing.T) {
	target := image.NewNRGBA(image.Rect(0, 0, 100, 60))
	background := image.NewNRGBA(image.Rect(0, 0, 90, 60))

	_, err := SlideComparisonImages(target, background)
	if err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("expected dimension mismatch, got %v", err)
	}
}

func TestSlideComparisonBytes(t *testing.T) {
	target, background := syntheticSlideImages()

	result, err := SlideComparisonBytes(encodePNG(t, target), encodePNG(t, background))
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetX != 51 || result.TargetY != 32 {
		t.Fatalf("target = %#v, want [51 32]", result)
	}
}

func syntheticSlideImages() (*image.NRGBA, *image.NRGBA) {
	background := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	draw.Draw(background, background.Bounds(), &image.Uniform{C: color.NRGBA{R: 245, G: 245, B: 245, A: 255}}, image.Point{}, draw.Src)

	target := image.NewNRGBA(background.Bounds())
	draw.Draw(target, target.Bounds(), background, image.Point{}, draw.Src)
	draw.Draw(target, image.Rect(40, 20, 62, 44), &image.Uniform{C: color.NRGBA{R: 20, G: 20, B: 20, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(target, image.Rect(90, 12, 93, 15), &image.Uniform{C: color.NRGBA{R: 20, G: 20, B: 20, A: 255}}, image.Point{}, draw.Src)

	return target, background
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
