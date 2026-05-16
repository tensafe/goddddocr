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

func TestSlideMatchImagesSimpleTarget(t *testing.T) {
	target, background := syntheticSlideMatchImages()

	result, err := SlideMatchImages(target, background, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetX != 73 || result.TargetY != 37 {
		t.Fatalf("target = %#v, want [73 37]", result)
	}
	if result.Confidence < 0.99 {
		t.Fatalf("confidence = %f, want near 1", result.Confidence)
	}
}

func TestSlideMatchImagesEdgeBased(t *testing.T) {
	target, background := syntheticSlideMatchImages()

	result, err := SlideMatchImages(target, background, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetX != 73 || result.TargetY != 37 {
		t.Fatalf("target = %#v, want [73 37]", result)
	}
	if result.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", result.Confidence)
	}
}

func TestSlideMatchBytes(t *testing.T) {
	target, background := syntheticSlideMatchImages()

	result, err := SlideMatchBytes(encodePNG(t, target), encodePNG(t, background), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetX != 73 || result.TargetY != 37 {
		t.Fatalf("target = %#v, want [73 37]", result)
	}
}

func TestSlideMatchImagesRejectsLargeTarget(t *testing.T) {
	target := image.NewNRGBA(image.Rect(0, 0, 100, 60))
	background := image.NewNRGBA(image.Rect(0, 0, 90, 60))

	_, err := SlideMatchImages(target, background, true)
	if err == nil || !strings.Contains(err.Error(), "exceed") {
		t.Fatalf("expected dimension error, got %v", err)
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

func syntheticSlideMatchImages() (*image.NRGBA, *image.NRGBA) {
	background := image.NewNRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < background.Bounds().Dy(); y++ {
		for x := 0; x < background.Bounds().Dx(); x++ {
			v := uint8((x*37 + y*17 + (x*y)%97) % 256)
			background.SetNRGBA(x, y, color.NRGBA{
				R: v,
				G: uint8((int(v)*3 + 45) % 256),
				B: uint8((int(v)*5 + 90) % 256),
				A: 255,
			})
		}
	}
	rect := image.Rect(58, 24, 88, 50)
	target := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(target, target.Bounds(), background, rect.Min, draw.Src)
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
