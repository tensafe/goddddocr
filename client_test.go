package goddddocr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOCRClientClassifyBytes(t *testing.T) {
	image := []byte("fake-image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			writeJSON(w, r, http.StatusOK, map[string]string{"status": "ready"})
		case "/ocr":
			var req ocrRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if !req.Confidence {
				t.Fatal("expected confidence request")
			}
			if !req.Probability {
				t.Fatal("expected probability request")
			}
			if len(req.ColorFilterColors) != 1 || req.ColorFilterColors[0] != "red" {
				t.Fatalf("unexpected color filter colors: %#v", req.ColorFilterColors)
			}
			if len(req.ColorFilterCustomRanges) != 1 || req.ColorFilterCustomRanges[0].Lower != [3]int{100, 50, 50} {
				t.Fatalf("unexpected color filter ranges: %#v", req.ColorFilterCustomRanges)
			}
			got, err := base64.StdEncoding.DecodeString(req.Image)
			if err != nil {
				t.Fatalf("decode image: %v", err)
			}
			if string(got) != string(image) {
				t.Fatalf("image mismatch: got %q, want %q", got, image)
			}
			confidence := 0.99
			writeJSON(w, r, http.StatusOK, ocrResponse{
				Result:           "3n3d",
				ProcessingTimeMS: 1.25,
				RequestID:        "req-1",
				Confidence:       &confidence,
				Probability: &ProbabilityMatrix{
					Text:        "3n3d",
					Charsets:    []string{"", "3", "n", "d"},
					Probability: [][]float64{{0.01, 0.97, 0.01, 0.01}},
					Confidence:  0.97,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewOCRClient(server.URL)
	if err := client.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := client.ClassifyBytes(context.Background(), image, &RemoteClassifyOptions{
		CharsetRange:            "3nd",
		ColorFilterColors:       []string{"red"},
		ColorFilterCustomRanges: []HSVRange{{Lower: [3]int{100, 50, 50}, Upper: [3]int{130, 255, 255}}},
		Confidence:              true,
		Probability:             true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "3n3d" {
		t.Fatalf("result mismatch: got %q", result.Result)
	}
	if result.RequestID != "req-1" {
		t.Fatalf("request id mismatch: got %q", result.RequestID)
	}
	if result.Confidence <= 0 {
		t.Fatalf("confidence not decoded: %f", result.Confidence)
	}
	if result.Probability == nil || result.Probability.Text != "3n3d" {
		t.Fatalf("probability not decoded: %+v", result.Probability)
	}
}

func TestOCRClientDetectBytes(t *testing.T) {
	image := []byte("fake-image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/det" {
			http.NotFound(w, r)
			return
		}
		var req detectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !req.Detailed {
			t.Fatal("expected detailed request")
		}
		got, err := base64.StdEncoding.DecodeString(req.Image)
		if err != nil {
			t.Fatalf("decode image: %v", err)
		}
		if string(got) != string(image) {
			t.Fatalf("image mismatch: got %q, want %q", got, image)
		}
		writeJSON(w, r, http.StatusOK, detectionResponse{
			Result:           [][]int{{1, 2, 30, 40}},
			Boxes:            []DetectionBox{{X1: 1, Y1: 2, X2: 30, Y2: 40, Score: 0.9}},
			ProcessingTimeMS: 1.5,
			RequestID:        "det-1",
		})
	}))
	defer server.Close()

	client := NewOCRClient(server.URL)
	result, err := client.DetectBytes(context.Background(), image, &RemoteDetectOptions{Detailed: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Result) != 1 || result.Result[0][2] != 30 {
		t.Fatalf("unexpected detection result: %#v", result.Result)
	}
	if len(result.Boxes) != 1 || result.Boxes[0].Score != 0.9 {
		t.Fatalf("unexpected detection boxes: %#v", result.Boxes)
	}
	if result.RequestID != "det-1" {
		t.Fatalf("request id mismatch: got %q", result.RequestID)
	}
}

func TestOCRClientSlideComparisonBytes(t *testing.T) {
	target := []byte("target-image")
	background := []byte("background-image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slide_comparison" {
			http.NotFound(w, r)
			return
		}
		var req slideComparisonRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotTarget, err := base64.StdEncoding.DecodeString(req.TargetImage)
		if err != nil {
			t.Fatalf("decode target: %v", err)
		}
		gotBackground, err := base64.StdEncoding.DecodeString(req.BackgroundImage)
		if err != nil {
			t.Fatalf("decode background: %v", err)
		}
		if string(gotTarget) != string(target) {
			t.Fatalf("target mismatch: got %q, want %q", gotTarget, target)
		}
		if string(gotBackground) != string(background) {
			t.Fatalf("background mismatch: got %q, want %q", gotBackground, background)
		}
		writeJSON(w, r, http.StatusOK, slideComparisonResponse{
			Result:           SlideResult{Target: []int{42, 18}, TargetX: 42, TargetY: 18},
			ProcessingTimeMS: 0.75,
			RequestID:        "slide-1",
		})
	}))
	defer server.Close()

	client := NewOCRClient(server.URL)
	result, err := client.SlideComparisonBytes(context.Background(), target, background)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result.TargetX != 42 || result.Result.TargetY != 18 {
		t.Fatalf("unexpected slide result: %#v", result.Result)
	}
	if result.RequestID != "slide-1" {
		t.Fatalf("request id mismatch: got %q", result.RequestID)
	}
}

func TestOCRClientRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusBadRequest, "invalid_image", "image must be valid base64")
	}))
	defer server.Close()

	client := NewOCRClient(server.URL)
	_, err := client.ClassifyBytes(context.Background(), []byte("x"), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var remoteErr *RemoteError
	if !errors.As(err, &remoteErr) {
		t.Fatalf("expected RemoteError, got %T", err)
	}
	if remoteErr.Code != "invalid_image" || remoteErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected remote error: %+v", remoteErr)
	}
}
