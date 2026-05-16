package goddddocr

import (
	"encoding/json"
	"image"
	"image/color"
	"testing"
)

func TestApplyColorFilterPreset(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})

	filtered, err := applyColorFilter(img, NewColorFilterColors("red"))
	if err != nil {
		t.Fatal(err)
	}

	if got := filtered.NRGBAAt(0, 0); got.R != 255 || got.G != 0 || got.B != 0 {
		t.Fatalf("red pixel changed: %+v", got)
	}
	if got := filtered.NRGBAAt(1, 0); got.R != 255 || got.G != 255 || got.B != 255 {
		t.Fatalf("non-red pixel should be white: %+v", got)
	}
}

func TestHSVRangeUnmarshal(t *testing.T) {
	var arrayRange HSVRange
	if err := json.Unmarshal([]byte(`[[100,50,50],[130,255,255]]`), &arrayRange); err != nil {
		t.Fatal(err)
	}
	if arrayRange.Lower != [3]int{100, 50, 50} || arrayRange.Upper != [3]int{130, 255, 255} {
		t.Fatalf("unexpected array range: %+v", arrayRange)
	}

	var objectRange HSVRange
	if err := json.Unmarshal([]byte(`{"lower":[0,0,0],"upper":[0,0,0]}`), &objectRange); err != nil {
		t.Fatal(err)
	}
	if objectRange.Lower != [3]int{0, 0, 0} || objectRange.Upper != [3]int{0, 0, 0} {
		t.Fatalf("unexpected object range: %+v", objectRange)
	}
}

func TestColorFilterValidation(t *testing.T) {
	if _, err := NewColorFilterColors("not-a-color").hsvRanges(); err == nil {
		t.Fatal("expected unsupported color error")
	}
	if _, err := NewColorFilterRanges(HSVRange{
		Lower: [3]int{181, 0, 0},
		Upper: [3]int{181, 255, 255},
	}).hsvRanges(); err == nil {
		t.Fatal("expected HSV validation error")
	}
}
