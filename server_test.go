package goddddocr

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestServerHealthWithoutEngine(t *testing.T) {
	s := NewServer(nil, WithLogger(nil))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %#v", body["status"])
	}
	if _, ok := body["model"]; ok {
		t.Fatalf("model should be omitted without engine: %#v", body["model"])
	}
}

func TestParseLogFormat(t *testing.T) {
	if got, err := ParseLogFormat("json"); err != nil || got != LogFormatJSON {
		t.Fatalf("ParseLogFormat(json) = %q, %v", got, err)
	}
	if got, err := ParseLogFormat(""); err != nil || got != LogFormatText {
		t.Fatalf("ParseLogFormat(empty) = %q, %v", got, err)
	}
	if _, err := ParseLogFormat("xml"); err == nil {
		t.Fatal("expected unsupported log format error")
	}
}

func TestServerJSONAccessLog(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	s := NewServer(nil, WithLogger(logger), WithLogFormat(LogFormatJSON))
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("User-Agent", "goddddocr-test")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected access log line")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("decode access log: %v\nline=%s", err, line)
	}
	if entry["event"] != "http_request" {
		t.Fatalf("event = %#v", entry["event"])
	}
	if entry["method"] != http.MethodGet {
		t.Fatalf("method = %#v", entry["method"])
	}
	if entry["path"] != "/ready" {
		t.Fatalf("path = %#v", entry["path"])
	}
	if entry["status"] != float64(http.StatusServiceUnavailable) {
		t.Fatalf("status = %#v", entry["status"])
	}
	if entry["user_agent"] != "goddddocr-test" {
		t.Fatalf("user_agent = %#v", entry["user_agent"])
	}
}
