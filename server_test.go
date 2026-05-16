package goddddocr

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeDetector struct {
	boxes []DetectionBox
	err   error
}

func (f fakeDetector) DetectBytesDetailed(data []byte) ([]DetectionBox, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.boxes, nil
}

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

func TestParseColorFilterFormValues(t *testing.T) {
	colors, err := parseColorFilterColorsFormValue("red, blue")
	if err != nil {
		t.Fatal(err)
	}
	if len(colors) != 2 || colors[0] != "red" || colors[1] != "blue" {
		t.Fatalf("colors = %#v", colors)
	}

	colors, err = parseColorFilterColorsFormValue(`["green","yellow"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(colors) != 2 || colors[0] != "green" || colors[1] != "yellow" {
		t.Fatalf("json colors = %#v", colors)
	}

	ranges, err := parseColorFilterRangesFormValue(`[[[100,50,50],[130,255,255]]]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || ranges[0].Lower != [3]int{100, 50, 50} {
		t.Fatalf("ranges = %#v", ranges)
	}

	if _, err := newColorFilterOptions([]string{"missing"}, nil); err == nil {
		t.Fatal("expected invalid color preset")
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
	if body["detection"] != false {
		t.Fatalf("detection = %#v, want false", body["detection"])
	}
}

func TestServerDetectionEndpoint(t *testing.T) {
	s := NewServer(nil, WithLogger(nil), WithDetector(fakeDetector{
		boxes: []DetectionBox{{X1: 1, Y1: 2, X2: 30, Y2: 40, Score: 0.9}},
	}))
	data := base64.StdEncoding.EncodeToString([]byte("fake-image"))
	body := strings.NewReader(`{"image":"` + data + `","detailed":true}`)
	req := httptest.NewRequest(http.MethodPost, "/det", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("det status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var resp detectionResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Result) != 1 || len(resp.Result[0]) != 4 {
		t.Fatalf("unexpected result: %#v", resp.Result)
	}
	if resp.Result[0][0] != 1 || resp.Result[0][3] != 40 {
		t.Fatalf("unexpected result box: %#v", resp.Result[0])
	}
	if len(resp.Boxes) != 1 || resp.Boxes[0].Score != 0.9 {
		t.Fatalf("detailed boxes missing: %#v", resp.Boxes)
	}
}

func TestServerDetectionEndpointNotReady(t *testing.T) {
	s := NewServer(nil, WithLogger(nil))
	data := base64.StdEncoding.EncodeToString([]byte("fake-image"))
	req := httptest.NewRequest(http.MethodPost, "/det", strings.NewReader(`{"image":"`+data+`"}`))
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("det status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestServerSlideComparisonEndpoint(t *testing.T) {
	target, background := syntheticSlideImages()
	payload, err := json.Marshal(slideComparisonRequest{
		TargetImage:     base64.StdEncoding.EncodeToString(encodePNG(t, target)),
		BackgroundImage: base64.StdEncoding.EncodeToString(encodePNG(t, background)),
	})
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, WithLogger(nil))
	req := httptest.NewRequest(http.MethodPost, "/slide_comparison", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("slide status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var resp slideComparisonResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.TargetX != 51 || resp.Result.TargetY != 32 {
		t.Fatalf("target = %#v, want [51 32]", resp.Result)
	}
	if resp.RequestID == "" {
		t.Fatal("request_id is empty")
	}
}

func TestServerSlideComparisonEndpointAlias(t *testing.T) {
	target, background := syntheticSlideImages()
	payload, err := json.Marshal(slideComparisonRequest{
		TargetImage:     base64.StdEncoding.EncodeToString(encodePNG(t, target)),
		BackgroundImage: base64.StdEncoding.EncodeToString(encodePNG(t, background)),
	})
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, WithLogger(nil))
	req := httptest.NewRequest(http.MethodPost, "/slide-comparison", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("slide alias status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestServerSlideComparisonEndpointMissingBackground(t *testing.T) {
	target, _ := syntheticSlideImages()
	payload, err := json.Marshal(slideComparisonRequest{
		TargetImage: base64.StdEncoding.EncodeToString(encodePNG(t, target)),
	})
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, WithLogger(nil))
	req := httptest.NewRequest(http.MethodPost, "/slide_comparison", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("slide status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var resp errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error.Code != "missing_background_image" {
		t.Fatalf("error code = %q", resp.Error.Code)
	}
}

func TestServerSlideComparisonFileEndpoint(t *testing.T) {
	target, background := syntheticSlideImages()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writeMultipartImage(t, writer, "target_file", "target.png", encodePNG(t, target))
	writeMultipartImage(t, writer, "background_file", "background.png", encodePNG(t, background))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, WithLogger(nil))
	req := httptest.NewRequest(http.MethodPost, "/slide_comparison/file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("slide file status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var resp slideComparisonResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.TargetX != 51 || resp.Result.TargetY != 32 {
		t.Fatalf("target = %#v, want [51 32]", resp.Result)
	}
}

func TestServerSlideMatchEndpoint(t *testing.T) {
	target, background := syntheticSlideMatchImages()
	payload, err := json.Marshal(slideMatchRequest{
		TargetImage:     base64.StdEncoding.EncodeToString(encodePNG(t, target)),
		BackgroundImage: base64.StdEncoding.EncodeToString(encodePNG(t, background)),
		SimpleTarget:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, WithLogger(nil))
	req := httptest.NewRequest(http.MethodPost, "/slide_match", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("slide match status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var resp slideMatchResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.TargetX != 73 || resp.Result.TargetY != 37 {
		t.Fatalf("target = %#v, want [73 37]", resp.Result)
	}
	if resp.Result.Confidence < 0.99 {
		t.Fatalf("confidence = %f, want near 1", resp.Result.Confidence)
	}
}

func TestServerSlideMatchEndpointAlias(t *testing.T) {
	target, background := syntheticSlideMatchImages()
	payload, err := json.Marshal(slideMatchRequest{
		TargetImage:     base64.StdEncoding.EncodeToString(encodePNG(t, target)),
		BackgroundImage: base64.StdEncoding.EncodeToString(encodePNG(t, background)),
		SimpleTarget:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, WithLogger(nil))
	req := httptest.NewRequest(http.MethodPost, "/slide-match", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("slide match alias status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func writeMultipartImage(t *testing.T, writer *multipart.Writer, fieldName string, fileName string, data []byte) {
	t.Helper()
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
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
