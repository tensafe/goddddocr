package goddddocr

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type goldenOCRFixture struct {
	Name          string  `json:"name"`
	Image         string  `json:"image"`
	Model         Model   `json:"model"`
	PythonDDDDOCR string  `json:"python_ddddocr,omitempty"`
	Expected      string  `json:"expected"`
	CharsetRange  string  `json:"charset_range,omitempty"`
	PNGFix        *bool   `json:"png_fix,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
	Source        string  `json:"source,omitempty"`
}

func TestGoldenOCRFixtures(t *testing.T) {
	fixtures := loadGoldenOCRFixtures(t)
	sessions := map[Model]*OCR{}
	defer func() {
		for _, session := range sessions {
			session.Close()
		}
	}()

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			model := fixture.Model
			if model == "" {
				model = ModelOld
			}

			ocr := sessions[model]
			if ocr == nil {
				var err error
				ocr, err = NewOCR(Config{Model: model})
				if err != nil {
					t.Skipf("ONNX Runtime unavailable for %s: %v", model, err)
				}
				sessions[model] = ocr
			}

			imagePath := cleanFixturePath(t, fixture.Image)
			data, err := os.ReadFile(imagePath)
			if err != nil {
				t.Fatalf("read fixture image %q: %v", imagePath, err)
			}

			result, err := ocr.ClassifyBytesDetailed(data, fixtureOptions(fixture))
			if err != nil {
				t.Fatal(err)
			}

			want := fixture.Expected
			if want == "" {
				want = fixture.PythonDDDDOCR
			}
			if result.Text != want {
				t.Fatalf("OCR mismatch for %s: got %q, want %q", fixture.Name, result.Text, want)
			}
			if fixture.PythonDDDDOCR != "" && result.Text != fixture.PythonDDDDOCR {
				t.Fatalf("Python parity mismatch for %s: got %q, python ddddocr %q", fixture.Name, result.Text, fixture.PythonDDDDOCR)
			}
			if fixture.MinConfidence > 0 && result.Confidence < fixture.MinConfidence {
				t.Fatalf("confidence too low for %s: got %.4f, want >= %.4f", fixture.Name, result.Confidence, fixture.MinConfidence)
			}
		})
	}
}

func loadGoldenOCRFixtures(t *testing.T) []goldenOCRFixture {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("fixtures", "ocr_golden.json"))
	if err != nil {
		t.Fatal(err)
	}

	var fixtures []goldenOCRFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("golden fixture manifest is empty")
	}
	for i, fixture := range fixtures {
		if fixture.Name == "" {
			t.Fatalf("fixture %d: missing name", i)
		}
		if fixture.Image == "" {
			t.Fatalf("fixture %q: missing image", fixture.Name)
		}
		if fixture.Expected == "" && fixture.PythonDDDDOCR == "" {
			t.Fatalf("fixture %q: missing expected output", fixture.Name)
		}
		if fixture.Expected != "" && fixture.PythonDDDDOCR != "" && fixture.Expected != fixture.PythonDDDDOCR {
			t.Fatalf("fixture %q: expected %q differs from python_ddddocr %q", fixture.Name, fixture.Expected, fixture.PythonDDDDOCR)
		}
	}

	return fixtures
}

func fixtureOptions(fixture goldenOCRFixture) *ClassifyOptions {
	var options ClassifyOptions
	hasOptions := false

	if fixture.PNGFix != nil {
		options.PNGFix = fixture.PNGFix
		hasOptions = true
	}
	if fixture.CharsetRange != "" {
		options.CharsetRange = NewCharsetRangeString(fixture.CharsetRange)
		hasOptions = true
	}
	if !hasOptions {
		return nil
	}
	return &options
}

func cleanFixturePath(t *testing.T, path string) string {
	t.Helper()

	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		t.Fatalf("fixture path must stay within repository: %q", path)
	}
	return cleaned
}
