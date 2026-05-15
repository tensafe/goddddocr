package goddddocr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	DefaultMaxImageBytes = 8 << 20
	DefaultMaxBodyBytes  = 12 << 20
)

type Server struct {
	ocr           OCREngine
	maxImageBytes int64
	maxBodyBytes  int64
	logger        *log.Logger
	requestSeq    atomic.Uint64
	metrics       *serverMetrics
}

type ServerOption func(*Server)

func WithMaxImageBytes(n int64) ServerOption {
	return func(s *Server) {
		if n > 0 {
			s.maxImageBytes = n
			bodyBytes := n*4/3 + 4096
			if bodyBytes > s.maxBodyBytes {
				s.maxBodyBytes = bodyBytes
			}
		}
	}
}

func WithMaxBodyBytes(n int64) ServerOption {
	return func(s *Server) {
		if n > 0 {
			s.maxBodyBytes = n
		}
	}
}

func WithLogger(logger *log.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

func NewServer(ocr OCREngine, options ...ServerOption) *Server {
	s := &Server{
		ocr:           ocr,
		maxImageBytes: DefaultMaxImageBytes,
		maxBodyBytes:  DefaultMaxBodyBytes,
		logger:        log.Default(),
		metrics:       newServerMetrics(time.Now()),
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("POST /ocr", s.handleOCR)
	mux.HandleFunc("POST /ocr/file", s.handleOCRFile)
	return s.accessLog(s.requestID(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}
	if s.ocr != nil {
		payload["model"] = s.ocr.Model()
	}
	writeJSON(w, r, http.StatusOK, payload)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.ocr == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "OCR engine is not initialized")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{
		"status": "ready",
		"model":  s.ocr.Model(),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, s.Metrics())
}

func (s *Server) Metrics() ServerMetricsSnapshot {
	if s == nil || s.metrics == nil {
		return ServerMetricsSnapshot{}
	}
	return s.metrics.snapshot(time.Now())
}

type ocrRequest struct {
	Image        string `json:"image"`
	PNGFix       *bool  `json:"png_fix,omitempty"`
	CharsetRange any    `json:"charset_range,omitempty"`
	Confidence   bool   `json:"confidence,omitempty"`
	Probability  bool   `json:"probability,omitempty"`
}

type ocrResponse struct {
	Result           string             `json:"result"`
	ProcessingTimeMS float64            `json:"processing_time_ms"`
	RequestID        string             `json:"request_id,omitempty"`
	Confidence       *float64           `json:"confidence,omitempty"`
	Probability      *ProbabilityMatrix `json:"probability,omitempty"`
}

func (s *Server) handleOCR(w http.ResponseWriter, r *http.Request) {
	if s.ocr == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "OCR engine is not initialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	defer r.Body.Close()

	var req ocrRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			writeError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", fmt.Sprintf("request exceeds %d bytes", s.maxBodyBytes))
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Image) == "" {
		writeError(w, r, http.StatusBadRequest, "missing_image", "image is required")
		return
	}

	data, err := decodeBase64Image(req.Image)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_image", err.Error())
		return
	}
	if int64(len(data)) > s.maxImageBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", fmt.Sprintf("image exceeds %d bytes", s.maxImageBytes))
		return
	}
	charsetRange, err := parseCharsetRangeValue(req.CharsetRange)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_charset_range", err.Error())
		return
	}

	start := time.Now()
	result, err := s.ocr.ClassifyBytesDetailed(data, &ClassifyOptions{
		PNGFix:       req.PNGFix,
		CharsetRange: charsetRange,
		Probability:  req.Probability,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "ocr_failed", err.Error())
		return
	}

	resp := ocrResponse{
		Result:           result.Text,
		ProcessingTimeMS: float64(time.Since(start).Microseconds()) / 1000.0,
		RequestID:        requestIDFrom(r),
	}
	if req.Confidence {
		resp.Confidence = &result.Confidence
	}
	if req.Probability {
		resp.Probability = result.Probability
	}
	writeJSON(w, r, http.StatusOK, resp)
}

func (s *Server) handleOCRFile(w http.ResponseWriter, r *http.Request) {
	if s.ocr == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "OCR engine is not initialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	defer r.Body.Close()

	if err := r.ParseMultipartForm(s.maxBodyBytes); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			writeError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", fmt.Sprintf("request exceeds %d bytes", s.maxBodyBytes))
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_multipart", "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "missing_file", "file is required")
		return
	}
	defer file.Close()

	data, err := readLimited(file, s.maxImageBytes)
	if err != nil {
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", err.Error())
		return
	}

	var pngFix *bool
	if v := strings.TrimSpace(r.FormValue("png_fix")); v != "" {
		parsed, err := parseBool(v)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_png_fix", "png_fix must be a boolean")
			return
		}
		pngFix = &parsed
	}
	charsetRange, err := parseCharsetRangeFormValue(r.FormValue("charset_range"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_charset_range", err.Error())
		return
	}
	confidence, err := parseOptionalBool(r.FormValue("confidence"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_confidence", "confidence must be a boolean")
		return
	}
	probability, err := parseOptionalBool(r.FormValue("probability"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_probability", "probability must be a boolean")
		return
	}

	start := time.Now()
	result, err := s.ocr.ClassifyBytesDetailed(data, &ClassifyOptions{
		PNGFix:       pngFix,
		CharsetRange: charsetRange,
		Probability:  probability,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "ocr_failed", err.Error())
		return
	}

	resp := ocrResponse{
		Result:           result.Text,
		ProcessingTimeMS: float64(time.Since(start).Microseconds()) / 1000.0,
		RequestID:        requestIDFrom(r),
	}
	if confidence {
		resp.Confidence = &result.Confidence
	}
	if probability {
		resp.Probability = result.Probability
	}
	writeJSON(w, r, http.StatusOK, resp)
}

func decodeBase64Image(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, ","); strings.HasPrefix(value, "data:") && idx >= 0 {
		value = value[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("image must be valid base64")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("image is empty")
	}
	return data, nil
}

func readLimited(src io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(src, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read image failed")
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("image exceeds %d bytes", maxBytes)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("image is empty")
	}
	return data, nil
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func parseOptionalBool(value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	return parseBool(value)
}

func parseCharsetRangeFormValue(value string) (*CharsetRange, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return NewCharsetRangeLimit(parsed), nil
	}
	if strings.HasPrefix(value, "[") {
		var chars []string
		if err := json.Unmarshal([]byte(value), &chars); err != nil {
			return nil, fmt.Errorf("charset_range array must contain strings")
		}
		return NewCharsetRangeChars(chars), nil
	}
	return NewCharsetRangeString(value), nil
}

func parseCharsetRangeValue(value any) (*CharsetRange, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case float64:
		if v < 0 || v != float64(int(v)) {
			return nil, fmt.Errorf("charset_range number must be a non-negative integer")
		}
		return NewCharsetRangeLimit(int(v)), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return NewCharsetRangeString(v), nil
	case []any:
		if len(v) == 0 {
			return nil, fmt.Errorf("charset_range array must not be empty")
		}
		chars := make([]string, 0, len(v))
		for _, item := range v {
			ch, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("charset_range array must contain strings")
			}
			chars = append(chars, ch)
		}
		return NewCharsetRangeChars(chars), nil
	default:
		return nil, fmt.Errorf("charset_range must be a number, string, or string array")
	}
}

type errorResponse struct {
	Error     responseError `json:"error"`
	RequestID string        `json:"request_id,omitempty"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if id := requestIDFrom(r); id != "" {
		w.Header().Set("X-Request-ID", id)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	writeJSON(w, r, status, errorResponse{
		Error: responseError{
			Code:    code,
			Message: message,
		},
		RequestID: requestIDFrom(r),
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		recordMetrics := r.URL == nil || r.URL.Path != "/metrics"
		if recordMetrics && s.metrics != nil {
			s.metrics.start()
		}
		defer func() {
			duration := time.Since(start)
			if recordMetrics && s.metrics != nil {
				s.metrics.finish(recorder.status, duration)
			}
			if s.logger != nil {
				s.logger.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%.3f remote=%s",
					requestIDFrom(r),
					r.Method,
					r.URL.Path,
					recorder.status,
					float64(duration.Microseconds())/1000.0,
					r.RemoteAddr,
				)
			}
		}()
		next.ServeHTTP(recorder, r)
	})
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if strings.TrimSpace(id) == "" {
			seq := s.requestSeq.Add(1)
			id = strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(seq, 36)
		}
		r.Header.Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func requestIDFrom(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.Header.Get("X-Request-ID")
}
