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
			got, err := base64.StdEncoding.DecodeString(req.Image)
			if err != nil {
				t.Fatalf("decode image: %v", err)
			}
			if string(got) != string(image) {
				t.Fatalf("image mismatch: got %q, want %q", got, image)
			}
			confidence := 0.99
			writeJSON(w, r, http.StatusOK, ocrResponse{Result: "3n3d", ProcessingTimeMS: 1.25, RequestID: "req-1", Confidence: &confidence})
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
		CharsetRange: "3nd",
		Confidence:   true,
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
