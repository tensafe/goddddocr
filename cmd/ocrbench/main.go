package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tensafe/goddddocr"
)

type benchConfig struct {
	BaseURL      string
	ImagePath    string
	Requests     int
	Concurrency  int
	Timeout      time.Duration
	CharsetRange string
	Expect       string
	Confidence   bool
	Probability  bool
	CheckReady   bool
	JSONOutput   bool
}

type benchResult struct {
	Duration time.Duration
	Text     string
	Error    string
	Mismatch bool
}

type benchSummary struct {
	BaseURL          string  `json:"base_url"`
	ImagePath        string  `json:"image_path"`
	Requests         int     `json:"requests"`
	Concurrency      int     `json:"concurrency"`
	Success          int     `json:"success"`
	Errors           int     `json:"errors"`
	Mismatches       int     `json:"mismatches"`
	ElapsedMS        float64 `json:"elapsed_ms"`
	QPS              float64 `json:"qps"`
	AverageLatencyMS float64 `json:"average_latency_ms"`
	MinLatencyMS     float64 `json:"min_latency_ms"`
	P50LatencyMS     float64 `json:"p50_latency_ms"`
	P95LatencyMS     float64 `json:"p95_latency_ms"`
	P99LatencyMS     float64 `json:"p99_latency_ms"`
	MaxLatencyMS     float64 `json:"max_latency_ms"`
	FirstError       string  `json:"first_error,omitempty"`
}

func main() {
	config := parseFlags()
	if err := validateConfig(config); err != nil {
		fmt.Fprintln(os.Stderr, "ocrbench:", err)
		os.Exit(2)
	}

	summary, err := runBench(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocrbench:", err)
		os.Exit(1)
	}
	if config.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(summary)
		return
	}
	printSummary(summary)
}

func parseFlags() benchConfig {
	var config benchConfig
	flag.StringVar(&config.BaseURL, "url", envString("GODDDDOCR_URL", "http://127.0.0.1:8088"), "goddddocr service root URL")
	flag.StringVar(&config.ImagePath, "image", "samples/yzm1.png", "image file to classify")
	flag.IntVar(&config.Requests, "requests", 100, "total requests to send")
	flag.IntVar(&config.Concurrency, "concurrency", 4, "number of concurrent clients")
	flag.DurationVar(&config.Timeout, "timeout", 10*time.Second, "per-request timeout")
	flag.StringVar(&config.CharsetRange, "charset-range", "", "optional ddddocr charset range")
	flag.StringVar(&config.Expect, "expect", "", "optional expected OCR text; mismatches are reported separately")
	flag.BoolVar(&config.Confidence, "confidence", true, "request confidence in responses")
	flag.BoolVar(&config.Probability, "probability", false, "request full probability matrix")
	flag.BoolVar(&config.CheckReady, "ready", true, "check /ready before running")
	flag.BoolVar(&config.JSONOutput, "json", false, "print JSON summary")
	flag.Parse()
	config.BaseURL = normalizeBaseURL(config.BaseURL)
	return config
}

func validateConfig(config benchConfig) error {
	if strings.TrimSpace(config.BaseURL) == "" {
		return fmt.Errorf("url is required")
	}
	if strings.TrimSpace(config.ImagePath) == "" {
		return fmt.Errorf("image is required")
	}
	if config.Requests <= 0 {
		return fmt.Errorf("requests must be positive")
	}
	if config.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

func runBench(config benchConfig) (benchSummary, error) {
	image, err := os.ReadFile(config.ImagePath)
	if err != nil {
		return benchSummary{}, fmt.Errorf("read image: %w", err)
	}

	client := goddddocr.NewOCRClient(config.BaseURL, goddddocr.WithClientTimeout(config.Timeout))
	if config.CheckReady {
		ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
		err := client.Ready(ctx)
		cancel()
		if err != nil {
			return benchSummary{}, fmt.Errorf("service is not ready: %w", err)
		}
	}

	options := &goddddocr.RemoteClassifyOptions{
		Confidence:  config.Confidence,
		Probability: config.Probability,
	}
	if strings.TrimSpace(config.CharsetRange) != "" {
		options.CharsetRange = config.CharsetRange
	}

	jobs := make(chan int)
	results := make(chan benchResult, config.Requests)
	workerCount := config.Concurrency
	if workerCount > config.Requests {
		workerCount = config.Requests
	}

	var wg sync.WaitGroup
	start := time.Now()
	for idx := 0; idx < workerCount; idx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				results <- runOneRequest(client, image, options, config.Expect)
			}
		}()
	}
	for idx := 0; idx < config.Requests; idx++ {
		jobs <- idx
	}
	close(jobs)
	wg.Wait()
	close(results)

	allResults := make([]benchResult, 0, config.Requests)
	for result := range results {
		allResults = append(allResults, result)
	}
	return summarizeBench(config, time.Since(start), allResults), nil
}

func runOneRequest(client *goddddocr.OCRClient, image []byte, options *goddddocr.RemoteClassifyOptions, expect string) benchResult {
	start := time.Now()
	result, err := client.ClassifyBytes(context.Background(), image, options)
	out := benchResult{Duration: time.Since(start)}
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Text = result.Result
	if expect != "" && result.Result != expect {
		out.Mismatch = true
		out.Error = fmt.Sprintf("result mismatch: got %q, want %q", result.Result, expect)
	}
	return out
}

func summarizeBench(config benchConfig, elapsed time.Duration, results []benchResult) benchSummary {
	latencies := make([]time.Duration, 0, len(results))
	var totalLatency time.Duration
	var firstError string
	summary := benchSummary{
		BaseURL:     config.BaseURL,
		ImagePath:   config.ImagePath,
		Requests:    config.Requests,
		Concurrency: config.Concurrency,
		ElapsedMS:   durationMS(elapsed),
	}

	for _, result := range results {
		latencies = append(latencies, result.Duration)
		totalLatency += result.Duration
		if result.Error != "" {
			summary.Errors++
			if firstError == "" {
				firstError = result.Error
			}
		} else {
			summary.Success++
		}
		if result.Mismatch {
			summary.Mismatches++
		}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	if elapsed > 0 {
		summary.QPS = float64(len(results)) / elapsed.Seconds()
	}
	if len(latencies) > 0 {
		summary.AverageLatencyMS = durationMS(totalLatency) / float64(len(latencies))
		summary.MinLatencyMS = durationMS(latencies[0])
		summary.P50LatencyMS = durationMS(percentile(latencies, 0.50))
		summary.P95LatencyMS = durationMS(percentile(latencies, 0.95))
		summary.P99LatencyMS = durationMS(percentile(latencies, 0.99))
		summary.MaxLatencyMS = durationMS(latencies[len(latencies)-1])
	}
	summary.FirstError = firstError
	return summary
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func durationMS(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000.0
}

func normalizeBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return value
	}
	switch strings.Trim(parsed.Path, "/") {
	case "health", "ready", "metrics", "ocr", "ocr/file":
		parsed.Path = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return strings.TrimRight(parsed.String(), "/")
	default:
		return value
	}
}

func printSummary(summary benchSummary) {
	fmt.Printf("goddddocr bench %s image=%s requests=%d concurrency=%d\n", summary.BaseURL, summary.ImagePath, summary.Requests, summary.Concurrency)
	fmt.Printf("success=%d errors=%d mismatches=%d elapsed_ms=%.3f qps=%.2f\n", summary.Success, summary.Errors, summary.Mismatches, summary.ElapsedMS, summary.QPS)
	fmt.Printf("latency_ms avg=%.3f min=%.3f p50=%.3f p95=%.3f p99=%.3f max=%.3f\n", summary.AverageLatencyMS, summary.MinLatencyMS, summary.P50LatencyMS, summary.P95LatencyMS, summary.P99LatencyMS, summary.MaxLatencyMS)
	if summary.FirstError != "" {
		fmt.Printf("first_error=%s\n", summary.FirstError)
	}
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
