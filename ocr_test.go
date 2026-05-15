package goddddocr

import (
	"os"
	"testing"
)

func TestClassifySample(t *testing.T) {
	ocr, err := NewOCR(Config{Model: ModelOld})
	if err != nil {
		t.Skipf("ONNX Runtime unavailable: %v", err)
	}
	defer ocr.Close()

	data, err := os.ReadFile("samples/yzm1.png")
	if err != nil {
		t.Fatal(err)
	}

	got, err := ocr.ClassifyBytes(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "3n3d" {
		t.Fatalf("sample OCR mismatch: got %q, want %q", got, "3n3d")
	}
}

func TestProcessOCROutputCharsetRange(t *testing.T) {
	charset := []string{"", "a", "b", "c"}
	data := []float32{
		0, 9, 1, 0,
		0, 0, 8, 1,
		0, 0, 1, 7,
	}
	valid := map[int]struct{}{0: {}, 1: {}, 3: {}}

	result, err := processOCROutput(data, []int64{3, 1, 4}, charset, valid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "ac" {
		t.Fatalf("range-filtered OCR mismatch: got %q, want %q", result.Text, "ac")
	}
	if result.Confidence <= 0 || result.Confidence > 1 {
		t.Fatalf("unexpected confidence: %f", result.Confidence)
	}
}

func TestCharsetRangeConstructors(t *testing.T) {
	ocr := &OCR{chars: []string{"", "a", "b", "c"}}

	valid, err := ocr.validIndices(&ClassifyOptions{CharsetRange: NewCharsetRangeString("ca")})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := valid[1]; !ok {
		t.Fatal("expected a to be valid")
	}
	if _, ok := valid[3]; !ok {
		t.Fatal("expected c to be valid")
	}
	if _, ok := valid[2]; ok {
		t.Fatal("did not expect b to be valid")
	}
}
