package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/tensafe/goddddocr"
)

type doctorConfig struct {
	Model             string
	ModelPath         string
	CharsetPath       string
	InputName         string
	OutputName        string
	SharedLibraryPath string
	PNGFix            bool
	ImagePath         string
	CharsetRange      string
	Expect            string
	JSONOutput        bool
}

type doctorReport struct {
	OK                 bool    `json:"ok"`
	Platform           string  `json:"platform"`
	Model              string  `json:"model"`
	ModelPath          string  `json:"model_path,omitempty"`
	CharsetPath        string  `json:"charset_path,omitempty"`
	InputName          string  `json:"input_name,omitempty"`
	OutputName         string  `json:"output_name,omitempty"`
	RuntimeLibraryPath string  `json:"runtime_library_path,omitempty"`
	CharsetSize        int     `json:"charset_size,omitempty"`
	ImagePath          string  `json:"image_path,omitempty"`
	Result             string  `json:"result,omitempty"`
	Expect             string  `json:"expect,omitempty"`
	Confidence         float64 `json:"confidence,omitempty"`
	ElapsedMS          float64 `json:"elapsed_ms,omitempty"`
	Error              string  `json:"error,omitempty"`
}

func main() {
	config, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocrdoctor:", err)
		os.Exit(2)
	}

	report := runDoctor(config)
	if config.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(report)
	} else {
		printReport(report)
	}
	if !report.OK {
		os.Exit(1)
	}
}

func parseFlags(args []string) (doctorConfig, error) {
	config := doctorConfig{}
	fs := flag.NewFlagSet("ocrdoctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&config.Model, "model", envString("GODDDDOCR_MODEL", string(goddddocr.ModelOld)), "OCR model: old, beta, or custom")
	fs.StringVar(&config.ModelPath, "model-path", envString("GODDDDOCR_MODEL_PATH", ""), "path to a custom ONNX OCR model")
	fs.StringVar(&config.CharsetPath, "charset-path", envString("GODDDDOCR_CHARSET_PATH", ""), "path to a custom charset JSON array")
	fs.StringVar(&config.InputName, "input-name", envString("GODDDDOCR_INPUT_NAME", ""), "ONNX input name override")
	fs.StringVar(&config.OutputName, "output-name", envString("GODDDDOCR_OUTPUT_NAME", ""), "ONNX output name override")
	fs.StringVar(&config.SharedLibraryPath, "onnxruntime-lib", envString("ONNXRUNTIME_SHARED_LIBRARY_PATH", ""), "path to ONNX Runtime shared library")
	fs.BoolVar(&config.PNGFix, "png-fix", envBool("GODDDDOCR_PNG_FIX", false), "composite transparent PNGs over a white background")
	fs.StringVar(&config.ImagePath, "image", "", "optional image file to classify")
	fs.StringVar(&config.CharsetRange, "charset-range", "", "optional ddddocr charset range for image classification")
	fs.StringVar(&config.Expect, "expect", "", "optional expected OCR text")
	fs.BoolVar(&config.JSONOutput, "json", false, "print JSON report")
	if err := fs.Parse(args); err != nil {
		return doctorConfig{}, err
	}
	return config, validateConfig(config)
}

func validateConfig(config doctorConfig) error {
	if strings.TrimSpace(config.Expect) != "" && strings.TrimSpace(config.ImagePath) == "" {
		return fmt.Errorf("expect requires image")
	}
	return nil
}

func runDoctor(config doctorConfig) doctorReport {
	report := doctorReport{
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		Model:       strings.TrimSpace(config.Model),
		ModelPath:   strings.TrimSpace(config.ModelPath),
		CharsetPath: strings.TrimSpace(config.CharsetPath),
		InputName:   strings.TrimSpace(config.InputName),
		OutputName:  strings.TrimSpace(config.OutputName),
		ImagePath:   strings.TrimSpace(config.ImagePath),
		Expect:      strings.TrimSpace(config.Expect),
	}

	ocr, err := goddddocr.NewOCR(goddddocr.Config{
		Model:             goddddocr.Model(report.Model),
		ModelPath:         report.ModelPath,
		CharsetPath:       report.CharsetPath,
		InputName:         report.InputName,
		OutputName:        report.OutputName,
		SharedLibraryPath: strings.TrimSpace(config.SharedLibraryPath),
		PNGFix:            config.PNGFix,
	})
	if err != nil {
		report.Error = err.Error()
		return report
	}
	defer ocr.Close()

	report.Model = string(ocr.Model())
	report.RuntimeLibraryPath = goddddocr.RuntimeLibraryPath()
	report.CharsetSize = len(ocr.Charset())
	if report.ImagePath == "" {
		report.OK = true
		return report
	}

	image, err := os.ReadFile(report.ImagePath)
	if err != nil {
		report.Error = fmt.Sprintf("read image: %v", err)
		return report
	}

	options := &goddddocr.ClassifyOptions{}
	if strings.TrimSpace(config.CharsetRange) != "" {
		options.CharsetRange = goddddocr.NewCharsetRangeString(config.CharsetRange)
	}
	start := time.Now()
	result, err := ocr.ClassifyBytesDetailed(image, options)
	report.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		report.Error = err.Error()
		return report
	}
	report.Result = result.Text
	report.Confidence = result.Confidence
	if report.Expect != "" && report.Result != report.Expect {
		report.Error = fmt.Sprintf("result mismatch: got %q, want %q", report.Result, report.Expect)
		return report
	}
	report.OK = true
	return report
}

func printReport(report doctorReport) {
	fmt.Println("goddddocr doctor")
	fmt.Println("platform:", report.Platform)
	if report.Model != "" {
		fmt.Println("model:", report.Model)
	}
	if report.ModelPath != "" {
		fmt.Println("model_path:", report.ModelPath)
	}
	if report.CharsetPath != "" {
		fmt.Println("charset_path:", report.CharsetPath)
	}
	if report.RuntimeLibraryPath != "" {
		fmt.Println("onnxruntime:", report.RuntimeLibraryPath)
	}
	if report.CharsetSize > 0 {
		fmt.Println("charset_size:", report.CharsetSize)
	}
	if report.ImagePath != "" {
		fmt.Println("image:", report.ImagePath)
	}
	if report.Result != "" || report.Expect != "" {
		fmt.Printf("result: %q confidence=%.6f elapsed_ms=%.3f\n", report.Result, report.Confidence, report.ElapsedMS)
	}
	if report.Error != "" {
		fmt.Println("status: failed")
		fmt.Println("error:", report.Error)
		return
	}
	fmt.Println("status: ok")
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
