package goddddocr

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"testing"
)

type goldenDetectionFixture struct {
	Name               string      `json:"name"`
	Image              string      `json:"image"`
	Expected           [][]int     `json:"expected"`
	PythonDDDDOCR      [][]int     `json:"python_ddddocr,omitempty"`
	MinScores          []float64   `json:"min_scores,omitempty"`
	MaxCoordinateDelta int         `json:"max_coordinate_delta,omitempty"`
	Config             detectorCfg `json:"config,omitempty"`
	Source             string      `json:"source,omitempty"`
}

type detectorCfg struct {
	InputSize      int     `json:"input_size,omitempty"`
	ScoreThreshold float64 `json:"score_threshold,omitempty"`
	NMSThreshold   float64 `json:"nms_threshold,omitempty"`
}

type goldenSlideFixture struct {
	Name          string  `json:"name"`
	Mode          string  `json:"mode"`
	Case          string  `json:"case,omitempty"`
	TargetImage   string  `json:"target_image,omitempty"`
	Background    string  `json:"background_image,omitempty"`
	SimpleTarget  bool    `json:"simple_target,omitempty"`
	Expected      []int   `json:"expected"`
	PythonDDDDOCR []int   `json:"python_ddddocr,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
	Source        string  `json:"source,omitempty"`
}

func TestGoldenDetectionFixtures(t *testing.T) {
	fixtures := loadGoldenDetectionFixtures(t)
	detectors := map[detectorCfg]*Detector{}
	defer func() {
		for _, detector := range detectors {
			_ = detector.Close()
		}
	}()

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			detector := detectors[fixture.Config]
			if detector == nil {
				var err error
				detector, err = NewDetector(DetectionConfig{
					InputSize:      fixture.Config.InputSize,
					ScoreThreshold: fixture.Config.ScoreThreshold,
					NMSThreshold:   fixture.Config.NMSThreshold,
				})
				if err != nil {
					t.Skipf("ONNX Runtime unavailable for detection fixture: %v", err)
				}
				detectors[fixture.Config] = detector
			}

			data, err := os.ReadFile(cleanFixturePath(t, fixture.Image))
			if err != nil {
				t.Fatalf("read fixture image: %v", err)
			}
			boxes, err := detector.DetectBytesDetailed(data)
			if err != nil {
				t.Fatal(err)
			}
			got := detectionRects(boxes)
			assertRectsNear(t, got, fixture.Expected, fixture.MaxCoordinateDelta)
			if len(fixture.PythonDDDDOCR) > 0 {
				assertRectsNear(t, got, fixture.PythonDDDDOCR, fixture.MaxCoordinateDelta)
			}
			if len(fixture.MinScores) > 0 {
				if len(boxes) != len(fixture.MinScores) {
					t.Fatalf("score fixture count = %d, boxes = %d", len(fixture.MinScores), len(boxes))
				}
				for idx, minScore := range fixture.MinScores {
					if boxes[idx].Score < minScore {
						t.Fatalf("box %d score = %.6f, want >= %.6f", idx, boxes[idx].Score, minScore)
					}
				}
			}
		})
	}
}

func TestGoldenSlideFixtures(t *testing.T) {
	fixtures := loadGoldenSlideFixtures(t)
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			target, background := loadGoldenSlideImages(t, fixture)

			var result *SlideResult
			var err error
			switch fixture.Mode {
			case "comparison":
				result, err = SlideComparisonImages(target, background)
			case "match":
				result, err = SlideMatchImages(target, background, fixture.SimpleTarget)
			default:
				t.Fatalf("unsupported slide mode %q", fixture.Mode)
			}
			if err != nil {
				t.Fatal(err)
			}
			assertPointEqual(t, result.Target, fixture.Expected)
			if len(fixture.PythonDDDDOCR) > 0 {
				assertPointEqual(t, result.Target, fixture.PythonDDDDOCR)
			}
			if fixture.MinConfidence > 0 && result.Confidence < fixture.MinConfidence {
				t.Fatalf("confidence = %.6f, want >= %.6f", result.Confidence, fixture.MinConfidence)
			}
		})
	}
}

func loadGoldenDetectionFixtures(t *testing.T) []goldenDetectionFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("fixtures", "detection_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []goldenDetectionFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("detection golden fixture manifest is empty")
	}
	for i, fixture := range fixtures {
		if fixture.Name == "" {
			t.Fatalf("detection fixture %d: missing name", i)
		}
		if fixture.Image == "" {
			t.Fatalf("detection fixture %q: missing image", fixture.Name)
		}
		if len(fixture.Expected) == 0 && len(fixture.PythonDDDDOCR) == 0 {
			t.Fatalf("detection fixture %q: missing expected boxes", fixture.Name)
		}
		if len(fixture.Expected) == 0 {
			fixtures[i].Expected = fixture.PythonDDDDOCR
		}
	}
	return fixtures
}

func loadGoldenSlideFixtures(t *testing.T) []goldenSlideFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("fixtures", "slide_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []goldenSlideFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("slide golden fixture manifest is empty")
	}
	for i, fixture := range fixtures {
		if fixture.Name == "" {
			t.Fatalf("slide fixture %d: missing name", i)
		}
		if fixture.Mode == "" {
			t.Fatalf("slide fixture %q: missing mode", fixture.Name)
		}
		if len(fixture.Expected) == 0 && len(fixture.PythonDDDDOCR) == 0 {
			t.Fatalf("slide fixture %q: missing expected target", fixture.Name)
		}
		if len(fixture.Expected) == 0 {
			fixtures[i].Expected = fixture.PythonDDDDOCR
		}
	}
	return fixtures
}

func loadGoldenSlideImages(t *testing.T, fixture goldenSlideFixture) (image.Image, image.Image) {
	t.Helper()
	switch fixture.Case {
	case "synthetic_comparison":
		return syntheticSlideImages()
	case "synthetic_match":
		return syntheticSlideMatchImages()
	case "":
		if fixture.TargetImage == "" || fixture.Background == "" {
			t.Fatalf("slide fixture %q needs target_image and background_image", fixture.Name)
		}
		targetData, err := os.ReadFile(cleanFixturePath(t, fixture.TargetImage))
		if err != nil {
			t.Fatalf("read target image: %v", err)
		}
		backgroundData, err := os.ReadFile(cleanFixturePath(t, fixture.Background))
		if err != nil {
			t.Fatalf("read background image: %v", err)
		}
		target, _, err := image.Decode(bytes.NewReader(targetData))
		if err != nil {
			t.Fatalf("decode target image: %v", err)
		}
		background, _, err := image.Decode(bytes.NewReader(backgroundData))
		if err != nil {
			t.Fatalf("decode background image: %v", err)
		}
		return target, background
	default:
		t.Fatalf("unsupported slide fixture case %q", fixture.Case)
		return nil, nil
	}
}

func detectionRects(boxes []DetectionBox) [][]int {
	rects := make([][]int, len(boxes))
	for idx, box := range boxes {
		rects[idx] = box.Rect()
	}
	return rects
}

func assertRectsNear(t *testing.T, got [][]int, want [][]int, maxDelta int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("box count = %d, want %d; got=%#v want=%#v", len(got), len(want), got, want)
	}
	for i := range want {
		if len(got[i]) != 4 || len(want[i]) != 4 {
			t.Fatalf("box %d shape mismatch: got=%#v want=%#v", i, got[i], want[i])
		}
		for j := 0; j < 4; j++ {
			if absInt(got[i][j]-want[i][j]) > maxDelta {
				t.Fatalf("box %d coord %d = %d, want %d ± %d; got=%#v want=%#v", i, j, got[i][j], want[i][j], maxDelta, got, want)
			}
		}
	}
}

func assertPointEqual(t *testing.T, got []int, want []int) {
	t.Helper()
	if len(got) != 2 || len(want) != 2 {
		t.Fatalf("point shape mismatch: got=%#v want=%#v", got, want)
	}
	if got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("point = %#v, want %#v", got, want)
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
