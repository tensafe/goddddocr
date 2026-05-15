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

	result, err := processOCROutput(data, []int64{3, 1, 4}, charset, valid, false)
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

func TestProcessOCROutputProbability(t *testing.T) {
	charset := []string{"", "a", "b"}
	data := []float32{
		0, 3, 1,
		0, 1, 4,
	}

	result, err := processOCROutput(data, []int64{2, 3}, charset, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "ab" {
		t.Fatalf("OCR mismatch: got %q, want %q", result.Text, "ab")
	}
	if result.Probability == nil {
		t.Fatal("expected probability matrix")
	}
	if result.Probability.Text != result.Text {
		t.Fatalf("probability text mismatch: got %q, want %q", result.Probability.Text, result.Text)
	}
	if len(result.Probability.Charsets) != len(charset) {
		t.Fatalf("charset length mismatch: got %d, want %d", len(result.Probability.Charsets), len(charset))
	}
	if len(result.Probability.Probability) != 2 || len(result.Probability.Probability[0]) != 3 {
		t.Fatalf("unexpected probability shape: %+v", result.Probability.Probability)
	}
	for rowIdx, row := range result.Probability.Probability {
		var sum float64
		for _, value := range row {
			sum += value
		}
		if sum < 0.999 || sum > 1.001 {
			t.Fatalf("probability row %d does not sum to 1: %.6f", rowIdx, sum)
		}
	}
	if result.Probability.Confidence <= 0 || result.Probability.Confidence > 1 {
		t.Fatalf("unexpected probability confidence: %f", result.Probability.Confidence)
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
