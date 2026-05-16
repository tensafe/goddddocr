package goddddocr

import (
	"image"
	"image/color"
	"math"
	"os"
	"testing"

	ort "github.com/yalue/onnxruntime_go"
)

func TestDetectSample(t *testing.T) {
	detector, err := NewDetector(DetectionConfig{})
	if err != nil {
		t.Skipf("ONNX Runtime unavailable: %v", err)
	}
	defer detector.Close()

	data, err := os.ReadFile("samples/yzm2.jpeg")
	if err != nil {
		t.Fatal(err)
	}
	boxes, err := detector.DetectBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes) == 0 {
		t.Fatal("expected at least one detection box")
	}
	for _, box := range boxes {
		if len(box) != 4 {
			t.Fatalf("unexpected box shape: %#v", box)
		}
		if box[0] < 0 || box[1] < 0 || box[2] <= box[0] || box[3] <= box[1] {
			t.Fatalf("invalid detection box: %#v", box)
		}
	}
}

func TestPreprocessDetectionImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 3, B: 7, A: 255})
		}
	}

	data, ratio, width, height, err := preprocessDetectionImage(img, 4)
	if err != nil {
		t.Fatal(err)
	}
	if ratio != 2 || width != 2 || height != 1 {
		t.Fatalf("metadata = ratio %.2f %dx%d", ratio, width, height)
	}
	if len(data) != 3*4*4 {
		t.Fatalf("data length = %d", len(data))
	}
	plane := 4 * 4
	if data[0] != 7 || data[plane] != 3 || data[2*plane] != 255 {
		t.Fatalf("expected BGR channel order, got B %.0f G %.0f R %.0f", data[0], data[plane], data[2*plane])
	}
	if data[2*4] != 114 || data[plane+2*4] != 114 || data[2*plane+2*4] != 114 {
		t.Fatalf("expected padded row to be 114")
	}
}

func TestProcessDetectionOutput(t *testing.T) {
	data := []float32{
		2.5, 2.5, float32(math.Log(2)), 0, 1, 0.9,
	}
	boxes, err := processDetectionOutput(data, ort.Shape{1, 1, 6}, 1, 100, 80, 416, 0.1, 0.45)
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes) != 1 {
		t.Fatalf("box count = %d, want 1", len(boxes))
	}
	box := boxes[0]
	if box.X1 != 11 || box.Y1 != 16 || box.X2 != 28 || box.Y2 != 24 {
		t.Fatalf("unexpected box: %#v", box)
	}
	if box.Score < 0.89 || box.Score > 0.91 {
		t.Fatalf("unexpected score: %.4f", box.Score)
	}
}

func TestProcessDetectionOutputThreshold(t *testing.T) {
	data := []float32{
		2.5, 2.5, float32(math.Log(2)), 0, 1, 0.08,
	}

	boxes, err := processDetectionOutput(data, ort.Shape{1, 1, 6}, 1, 100, 80, 416, 0.1, 0.45)
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes) != 0 {
		t.Fatalf("box count = %d, want 0", len(boxes))
	}

	boxes, err = processDetectionOutput(data, ort.Shape{1, 1, 6}, 1, 100, 80, 416, 0.05, 0.45)
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes) != 1 {
		t.Fatalf("box count = %d, want 1", len(boxes))
	}
}

func TestDetectorThresholdOptions(t *testing.T) {
	detector := &Detector{scoreThreshold: 0.1, nmsThreshold: 0.45}
	scoreThreshold := 0.05
	nmsThreshold := 0.35

	score, nms, err := detector.thresholds(&DetectionOptions{
		ScoreThreshold: &scoreThreshold,
		NMSThreshold:   &nmsThreshold,
	})
	if err != nil {
		t.Fatal(err)
	}
	if score != scoreThreshold || nms != nmsThreshold {
		t.Fatalf("thresholds = %.2f %.2f", score, nms)
	}

	invalid := 1.5
	if _, _, err := detector.thresholds(&DetectionOptions{ScoreThreshold: &invalid}); err == nil {
		t.Fatal("expected invalid threshold error")
	}
}

func TestNMSDetectionCandidates(t *testing.T) {
	candidates := []detectionCandidate{
		{x1: 0, y1: 0, x2: 10, y2: 10, score: 0.9},
		{x1: 1, y1: 1, x2: 11, y2: 11, score: 0.8},
		{x1: 30, y1: 30, x2: 40, y2: 40, score: 0.7},
	}
	kept := nmsDetectionCandidates(candidates, 0.45)
	if len(kept) != 2 {
		t.Fatalf("kept = %d, want 2", len(kept))
	}
	if kept[0].score != 0.9 || kept[1].score != 0.7 {
		t.Fatalf("unexpected kept candidates: %#v", kept)
	}
}

func TestDetectionRowsRejectsInvalidShape(t *testing.T) {
	if _, _, err := detectionRows([]float32{1, 2, 3}, ort.Shape{1, 3}); err == nil {
		t.Fatal("expected invalid shape error")
	}
}
