package main

import (
	"context"
	"errors"
	"flag"
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
	model := flag.String("model", envString("GODDDDOCR_MODEL", string(goddddocr.ModelOld)), "OCR model: old or beta")
	ortLib := flag.String("onnxruntime-lib", envString("ONNXRUNTIME_SHARED_LIBRARY_PATH", ""), "path to ONNX Runtime shared library")
	pngFix := flag.Bool("png-fix", envBool("GODDDDOCR_PNG_FIX", false), "composite transparent PNGs over a white background")
	maxImageBytes := flag.Int64("max-image-bytes", envInt64("GODDDDOCR_MAX_IMAGE_BYTES", goddddocr.DefaultMaxImageBytes), "maximum decoded image size in bytes")
	shutdownTimeout := flag.Duration("shutdown-timeout", envDuration("GODDDDOCR_SHUTDOWN_TIMEOUT", 10*time.Second), "graceful shutdown timeout")
	flag.Parse()

	ocr, err := goddddocr.NewOCR(goddddocr.Config{
		Model:             goddddocr.Model(*model),
		SharedLibraryPath: *ortLib,
		PNGFix:            *pngFix,
	})
	if err != nil {
		log.Fatalf("init OCR: %v", err)
	}
	defer ocr.Close()

	server := &http.Server{
		Addr: *addr,
		Handler: goddddocr.NewServer(
			ocr,
			goddddocr.WithMaxImageBytes(*maxImageBytes),
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
			log.Printf("server shutdown failed: %v", err)
		}
	}()

	log.Printf("goddddocr server listening on %s, model=%s", *addr, ocr.Model())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	log.Printf("goddddocr server stopped")
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
