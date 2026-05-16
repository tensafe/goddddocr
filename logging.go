package goddddocr

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

func ParseLogFormat(value string) (LogFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(LogFormatText):
		return LogFormatText, nil
	case string(LogFormatJSON):
		return LogFormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported log format %q", value)
	}
}

func WithLogFormat(format LogFormat) ServerOption {
	return func(s *Server) {
		if format != "" {
			s.logFormat = format
		}
	}
}

type accessLogEntry struct {
	Time       string  `json:"time"`
	Level      string  `json:"level"`
	Event      string  `json:"event"`
	RequestID  string  `json:"request_id,omitempty"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Status     int     `json:"status"`
	DurationMS float64 `json:"duration_ms"`
	Remote     string  `json:"remote,omitempty"`
	UserAgent  string  `json:"user_agent,omitempty"`
}

func writeAccessLog(logger *log.Logger, format LogFormat, r *http.Request, status int, duration time.Duration) {
	if logger == nil || r == nil {
		return
	}
	durationMS := float64(duration.Microseconds()) / 1000.0
	if format == LogFormatJSON {
		entry := accessLogEntry{
			Time:       time.Now().UTC().Format(time.RFC3339Nano),
			Level:      "info",
			Event:      "http_request",
			RequestID:  requestIDFrom(r),
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     status,
			DurationMS: durationMS,
			Remote:     r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
		data, err := json.Marshal(entry)
		if err != nil {
			logger.Printf("event=http_request log_error=%q", err)
			return
		}
		logger.Print(string(data))
		return
	}

	logger.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%.3f remote=%s",
		requestIDFrom(r),
		r.Method,
		r.URL.Path,
		status,
		durationMS,
		r.RemoteAddr,
	)
}
