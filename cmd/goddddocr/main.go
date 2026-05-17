package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/tensafe/goddddocr"
)

type ocrCommandConfig struct {
	Model             string
	ModelPath         string
	CharsetPath       string
	InputName         string
	OutputName        string
	SharedLibraryPath string
	PNGFix            bool
	FilePath          string
	CharsetRange      string
	Expect            string
	JSONOutput        bool
	Confidence        bool
	Probability       bool
}

type ocrCommandReport struct {
	OK                 bool                         `json:"ok"`
	Platform           string                       `json:"platform,omitempty"`
	Model              string                       `json:"model,omitempty"`
	ModelPath          string                       `json:"model_path,omitempty"`
	CharsetPath        string                       `json:"charset_path,omitempty"`
	InputName          string                       `json:"input_name,omitempty"`
	OutputName         string                       `json:"output_name,omitempty"`
	RuntimeLibraryPath string                       `json:"runtime_library_path,omitempty"`
	CharsetSize        int                          `json:"charset_size,omitempty"`
	ImagePath          string                       `json:"image_path,omitempty"`
	Result             string                       `json:"result,omitempty"`
	Text               string                       `json:"text,omitempty"`
	Expect             string                       `json:"expect,omitempty"`
	Confidence         *float64                     `json:"confidence,omitempty"`
	Probability        *goddddocr.ProbabilityMatrix `json:"probability,omitempty"`
	ProcessingTimeMS   float64                      `json:"processing_time_ms,omitempty"`
	ElapsedMS          float64                      `json:"elapsed_ms,omitempty"`
	Error              string                       `json:"error,omitempty"`
}

func main() {
	os.Exit(runRootCommand(os.Args[1:], os.Stdout, os.Stderr))
}

func runRootCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printRootUsage(stderr)
		return 2
	}

	switch args[0] {
	case "ocr":
		return runOCRCommand(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printRootUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "goddddocr: unknown command %q\n\n", args[0])
		printRootUsage(stderr)
		return 2
	}
}

func printRootUsage(out io.Writer) {
	fmt.Fprintln(out, "usage: goddddocr <command> [options]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "commands:")
	fmt.Fprintln(out, "  ocr    recognize one local captcha image")
}

func runOCRCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	config, err := parseOCRCommandFlags(args, stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(stderr, "goddddocr ocr:", err)
		return 2
	}

	report := executeOCRCommand(config)
	if config.JSONOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(report)
	} else {
		printOCRCommandReport(stdout, report, config)
	}
	if !report.OK {
		return 1
	}
	return 0
}

func parseOCRCommandFlags(args []string, output io.Writer) (ocrCommandConfig, error) {
	config := ocrCommandConfig{}
	fs := flag.NewFlagSet("goddddocr ocr", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&config.Model, "model", envString("GODDDDOCR_MODEL", string(goddddocr.ModelOld)), "OCR model: old, beta, or custom")
	fs.StringVar(&config.ModelPath, "model-path", envString("GODDDDOCR_MODEL_PATH", ""), "path to a custom ONNX OCR model")
	fs.StringVar(&config.CharsetPath, "charset-path", envString("GODDDDOCR_CHARSET_PATH", ""), "path to a custom charset JSON array")
	fs.StringVar(&config.InputName, "input-name", envString("GODDDDOCR_INPUT_NAME", ""), "ONNX input name override")
	fs.StringVar(&config.OutputName, "output-name", envString("GODDDDOCR_OUTPUT_NAME", ""), "ONNX output name override")
	fs.StringVar(&config.SharedLibraryPath, "onnxruntime-lib", envString("ONNXRUNTIME_SHARED_LIBRARY_PATH", ""), "path to ONNX Runtime shared library")
	fs.BoolVar(&config.PNGFix, "png-fix", envBool("GODDDDOCR_PNG_FIX", false), "composite transparent PNGs over a white background")
	fs.StringVar(&config.FilePath, "file", "", "image file to classify")
	fs.StringVar(&config.FilePath, "image", "", "alias for --file")
	fs.StringVar(&config.CharsetRange, "charset-range", "", "optional ddddocr charset range for image classification")
	fs.StringVar(&config.Expect, "expect", "", "optional expected OCR text")
	fs.BoolVar(&config.JSONOutput, "json", false, "print JSON result")
	fs.BoolVar(&config.Confidence, "confidence", false, "include confidence in the result")
	fs.BoolVar(&config.Probability, "probability", false, "include full probability matrix in JSON output")
	if err := fs.Parse(args); err != nil {
		return ocrCommandConfig{}, err
	}
	if fs.NArg() > 0 {
		return ocrCommandConfig{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	return config, validateOCRCommandConfig(config)
}

func validateOCRCommandConfig(config ocrCommandConfig) error {
	if strings.TrimSpace(config.FilePath) == "" {
		return fmt.Errorf("file is required")
	}
	return nil
}

func executeOCRCommand(config ocrCommandConfig) ocrCommandReport {
	report := ocrCommandReport{
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		Model:       strings.TrimSpace(config.Model),
		ModelPath:   strings.TrimSpace(config.ModelPath),
		CharsetPath: strings.TrimSpace(config.CharsetPath),
		InputName:   strings.TrimSpace(config.InputName),
		OutputName:  strings.TrimSpace(config.OutputName),
		ImagePath:   strings.TrimSpace(config.FilePath),
		Expect:      strings.TrimSpace(config.Expect),
	}

	image, err := os.ReadFile(report.ImagePath)
	if err != nil {
		report.Error = fmt.Sprintf("read image: %v", err)
		return report
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

	options := &goddddocr.ClassifyOptions{Probability: config.Probability}
	if strings.TrimSpace(config.CharsetRange) != "" {
		options.CharsetRange = goddddocr.NewCharsetRangeString(config.CharsetRange)
	}

	start := time.Now()
	result, err := ocr.ClassifyBytesDetailed(image, options)
	elapsedMS := float64(time.Since(start).Microseconds()) / 1000.0
	report.ProcessingTimeMS = elapsedMS
	report.ElapsedMS = elapsedMS
	if err != nil {
		report.Error = err.Error()
		return report
	}

	report.Result = result.Text
	report.Text = result.Text
	if config.Confidence || config.Probability {
		confidence := result.Confidence
		report.Confidence = &confidence
	}
	if config.Probability {
		report.Probability = result.Probability
	}
	if report.Expect != "" && report.Result != report.Expect {
		report.Error = fmt.Sprintf("result mismatch: got %q, want %q", report.Result, report.Expect)
		return report
	}
	report.OK = true
	return report
}

func printOCRCommandReport(out io.Writer, report ocrCommandReport, config ocrCommandConfig) {
	if report.Error != "" {
		fmt.Fprintln(out, "status: failed")
		fmt.Fprintln(out, "error:", report.Error)
		return
	}
	if config.Confidence || config.Probability {
		confidence := 0.0
		if report.Confidence != nil {
			confidence = *report.Confidence
		}
		fmt.Fprintf(out, "%s confidence=%.6f processing_time_ms=%.3f\n", report.Result, confidence, report.ProcessingTimeMS)
		return
	}
	fmt.Fprintln(out, report.Result)
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
