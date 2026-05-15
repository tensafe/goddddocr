package goddddocr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	ocr *OCR
}

func NewServer(ocr *OCR) *Server {
	return &Server{ocr: ocr}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /ocr", s.handleOCR)
	mux.HandleFunc("POST /ocr/file", s.handleOCRFile)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"model":  s.ocr.Model(),
		"time":   time.Now().Format(time.RFC3339),
	})
}

type ocrRequest struct {
	Image  string `json:"image"`
	PNGFix *bool  `json:"png_fix,omitempty"`
}

type ocrResponse struct {
	Result           string  `json:"result"`
	ProcessingTimeMS float64 `json:"processing_time_ms"`
}

func (s *Server) handleOCR(w http.ResponseWriter, r *http.Request) {
	var req ocrRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Image) == "" {
		writeError(w, http.StatusBadRequest, "image is required")
		return
	}

	data, err := decodeBase64Image(req.Image)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	start := time.Now()
	result, err := s.ocr.ClassifyBytes(data, &ClassifyOptions{PNGFix: req.PNGFix})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ocrResponse{
		Result:           result,
		ProcessingTimeMS: float64(time.Since(start).Microseconds()) / 1000.0,
	})
}

func (s *Server) handleOCRFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read uploaded file failed")
		return
	}

	var pngFix *bool
	if v := strings.TrimSpace(r.FormValue("png_fix")); v != "" {
		parsed := v == "true" || v == "1" || strings.EqualFold(v, "yes")
		pngFix = &parsed
	}

	start := time.Now()
	result, err := s.ocr.ClassifyBytes(data, &ClassifyOptions{PNGFix: pngFix})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ocrResponse{
		Result:           result,
		ProcessingTimeMS: float64(time.Since(start).Microseconds()) / 1000.0,
	})
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
