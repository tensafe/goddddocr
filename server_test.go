package goddddocr

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeBase64ImageDataURL(t *testing.T) {
	want := []byte("image")
	got, err := decodeBase64Image("data:image/png;base64," + base64.StdEncoding.EncodeToString(want))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("decoded mismatch: got %q, want %q", got, want)
	}
}

func TestParseBool(t *testing.T) {
	truthy, err := parseBool("yes")
	if err != nil || !truthy {
		t.Fatalf("yes should parse true, got %v %v", truthy, err)
	}

	falsey, err := parseBool("0")
	if err != nil || falsey {
		t.Fatalf("0 should parse false, got %v %v", falsey, err)
	}

	if _, err := parseBool("maybe"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseCharsetRangeValue(t *testing.T) {
	rng, err := parseCharsetRangeValue("abc")
	if err != nil {
		t.Fatal(err)
	}
	if rng == nil || len(rng.chars) != 3 {
		t.Fatalf("unexpected string range: %+v", rng)
	}

	rng, err = parseCharsetRangeValue(float64(2))
	if err != nil {
		t.Fatal(err)
	}
	if rng == nil || rng.limit == nil || *rng.limit != 2 {
		t.Fatalf("unexpected numeric range: %+v", rng)
	}

	rng, err = parseCharsetRangeValue([]any{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if rng == nil || len(rng.chars) != 2 {
		t.Fatalf("unexpected list range: %+v", rng)
	}
}

func TestServerMetricsEndpoint(t *testing.T) {
	s := NewServer(nil, WithLogger(nil))
	server := httptest.NewServer(s.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	resp, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var snapshot ServerMetricsSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.TotalRequests != 1 {
		t.Fatalf("total_requests = %d, want 1", snapshot.TotalRequests)
	}
	if snapshot.CompletedRequests != 1 {
		t.Fatalf("completed_requests = %d, want 1", snapshot.CompletedRequests)
	}
	if snapshot.ErrorRequests != 1 {
		t.Fatalf("error_requests = %d, want 1", snapshot.ErrorRequests)
	}
	if snapshot.StatusCodes["503"] != 1 {
		t.Fatalf("status_codes = %#v, want 503 count 1", snapshot.StatusCodes)
	}
	if snapshot.InFlightRequests != 0 {
		t.Fatalf("in_flight_requests = %d, want 0", snapshot.InFlightRequests)
	}
	if snapshot.StartedAt == "" {
		t.Fatal("started_at is empty")
	}
}
