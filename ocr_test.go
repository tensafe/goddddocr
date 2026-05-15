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
