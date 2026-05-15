package goddddocr

import (
	"os"
	"testing"
)

func TestNewOCRPoolRejectsInvalidWorkerCount(t *testing.T) {
	if _, err := NewOCRPool(Config{Model: ModelOld}, 0); err == nil {
		t.Fatal("expected invalid worker count error")
	}
}

func TestOCRPoolClassifySample(t *testing.T) {
	pool, err := NewOCRPool(Config{Model: ModelOld}, 2)
	if err != nil {
		t.Skipf("ONNX Runtime unavailable: %v", err)
	}
	defer pool.Close()

	if pool.Size() != 2 {
		t.Fatalf("pool size = %d, want 2", pool.Size())
	}
	if pool.Model() != ModelOld {
		t.Fatalf("pool model = %q, want %q", pool.Model(), ModelOld)
	}

	data, err := os.ReadFile("samples/yzm1.png")
	if err != nil {
		t.Fatal(err)
	}
	for idx := 0; idx < 2; idx++ {
		result, err := pool.ClassifyBytesDetailed(data, &ClassifyOptions{
			CharsetRange: NewCharsetRangeString("0123456789abcdefghijklmnopqrstuvwxyz"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Text != "3n3d" {
			t.Fatalf("sample OCR mismatch: got %q, want %q", result.Text, "3n3d")
		}
	}
}
