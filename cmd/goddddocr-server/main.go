package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/tensafe/goddddocr"
)

func main() {
	addr := flag.String("addr", envString("GODDDDOCR_ADDR", ":8088"), "HTTP listen address")
	model := flag.String("model", envString("GODDDDOCR_MODEL", string(goddddocr.ModelOld)), "OCR model: old, beta, or custom")
	modelPath := flag.String("model-path", envString("GODDDDOCR_MODEL_PATH", ""), "path to a custom ONNX OCR model")
	charsetPath := flag.String("charset-path", envString("GODDDDOCR_CHARSET_PATH", ""), "path to a custom charset JSON array")
	inputName := flag.String("input-name", envString("GODDDDOCR_INPUT_NAME", ""), "ONNX input name override")
	outputName := flag.String("output-name", envString("GODDDDOCR_OUTPUT_NAME", ""), "ONNX output name override")
	ortLib := flag.String("onnxruntime-lib", envString("ONNXRUNTIME_SHARED_LIBRARY_PATH", ""), "path to ONNX Runtime shared library")
	pngFix := flag.Bool("png-fix", envBool("GODDDDOCR_PNG_FIX", false), "composite transparent PNGs over a white background")
	workers := flag.Int("workers", envInt("GODDDDOCR_WORKERS", 1), "number of OCR sessions to keep in the worker pool")
	logFormatValue := flag.String("log-format", envString("GODDDDOCR_LOG_FORMAT", string(goddddocr.LogFormatText)), "log format: text or json")
	maxImageBytes := flag.Int64("max-image-bytes", envInt64("GODDDDOCR_MAX_IMAGE_BYTES", goddddocr.DefaultMaxImageBytes), "maximum decoded image size in bytes")
	shutdownTimeout := flag.Duration("shutdown-timeout", envDuration("GODDDDOCR_SHUTDOWN_TIMEOUT", 10*time.Second), "graceful shutdown timeout")
	flag.Parse()

	logFormat, err := goddddocr.ParseLogFormat(*logFormatValue)
	if err != nil {
		log.Fatal(err)
	}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	if logFormat == goddddocr.LogFormatJSON {
		logger = log.New(os.Stdout, "", 0)
	}
	if *workers <= 0 {
		logger.Fatalf("workers must be positive")
	}

	ocr, err := goddddocr.NewOCRPool(goddddocr.Config{
		Model:             goddddocr.Model(*model),
		ModelPath:         *modelPath,
		CharsetPath:       *charsetPath,
		InputName:         *inputName,
		OutputName:        *outputName,
		SharedLibraryPath: *ortLib,
		PNGFix:            *pngFix,
	}, *workers)
	if err != nil {
		logServiceEvent(logger, logFormat, "server_init_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer ocr.Close()

	server := &http.Server{
		Addr: *addr,
		Handler: goddddocr.NewServer(
			ocr,
			goddddocr.WithMaxImageBytes(*maxImageBytes),
			goddddocr.WithLogger(logger),
			goddddocr.WithLogFormat(logFormat),
		).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logServiceEvent(logger, logFormat, "server_shutdown_failed", map[string]any{"error": err.Error()})
		}
	}()

	startFields := map[string]any{
		"addr":       *addr,
		"model":      ocr.Model(),
		"workers":    ocr.Size(),
		"log_format": logFormat,
	}
	if strings.TrimSpace(*modelPath) != "" {
		startFields["model_path"] = *modelPath
	}
	if strings.TrimSpace(*charsetPath) != "" {
		startFields["charset_path"] = *charsetPath
	}
	logServiceEvent(logger, logFormat, "server_started", startFields)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logServiceEvent(logger, logFormat, "server_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	logServiceEvent(logger, logFormat, "server_stopped", nil)
}

func logServiceEvent(logger *log.Logger, format goddddocr.LogFormat, event string, fields map[string]any) {
	if logger == nil {
		return
	}
	if format == goddddocr.LogFormatJSON {
		payload := map[string]any{
			"time":  time.Now().UTC().Format(time.RFC3339Nano),
			"level": "info",
			"event": event,
		}
		for key, value := range fields {
			payload[key] = value
		}
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Printf(`{"level":"error","event":"log_encode_failed","error":%q}`, err.Error())
			return
		}
		logger.Print(string(data))
		return
	}
	if len(fields) == 0 {
		logger.Printf("event=%s", event)
		return
	}
	parts := make([]string, 0, len(fields)+1)
	parts = append(parts, "event="+event)
	for key, value := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}
	logger.Print(strings.Join(parts, " "))
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "true", "1", "yes", "y", "on":
		return true
	case "false", "0", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func envInt64(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
