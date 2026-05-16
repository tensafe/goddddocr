package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tensafe/goddddocr"
)

type evalConfig struct {
	ManifestPath      string
	DirPath           string
	Recursive         bool
	ExpectedFrom      string
	Model             string
	ModelPath         string
	CharsetPath       string
	InputName         string
	OutputName        string
	SharedLibraryPath string
	PNGFix            bool
	CharsetRange      string
	ColorFilterColors string
	JSONOutput        bool
	CSVPath           string
	MarkdownPath      string
	FailOnMismatch    bool
}

type evalSample struct {
	Name          string               `json:"name"`
	Image         string               `json:"image"`
	Model         goddddocr.Model      `json:"model,omitempty"`
	Expected      string               `json:"expected,omitempty"`
	PythonDDDDOCR string               `json:"python_ddddocr,omitempty"`
	CharsetRange  string               `json:"charset_range,omitempty"`
	Colors        []string             `json:"color_filter_colors,omitempty"`
	Ranges        []goddddocr.HSVRange `json:"color_filter_custom_ranges,omitempty"`
	PNGFix        *bool                `json:"png_fix,omitempty"`
	Source        string               `json:"source,omitempty"`
}

type evalResult struct {
	Name       string  `json:"name"`
	Image      string  `json:"image"`
	Model      string  `json:"model"`
	Expected   string  `json:"expected,omitempty"`
	Labeled    bool    `json:"labeled"`
	Result     string  `json:"result,omitempty"`
	Matched    bool    `json:"matched"`
	Confidence float64 `json:"confidence,omitempty"`
	ElapsedMS  float64 `json:"elapsed_ms,omitempty"`
	Error      string  `json:"error,omitempty"`
}

type evalSummary struct {
	Total      int          `json:"total"`
	Labeled    int          `json:"labeled"`
	Unlabeled  int          `json:"unlabeled"`
	Matched    int          `json:"matched"`
	Mismatched int          `json:"mismatched"`
	Errors     int          `json:"errors"`
	Accuracy   float64      `json:"accuracy"`
	Results    []evalResult `json:"results"`
}

func main() {
	config, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocreval:", err)
		os.Exit(2)
	}

	samples, err := loadSamples(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocreval:", err)
		os.Exit(1)
	}
	summary, err := runEval(config, samples)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocreval:", err)
		os.Exit(1)
	}

	if config.CSVPath != "" {
		if err := writeCSV(config.CSVPath, summary.Results); err != nil {
			fmt.Fprintln(os.Stderr, "ocreval:", err)
			os.Exit(1)
		}
	}
	if config.MarkdownPath != "" {
		if err := writeMarkdown(config.MarkdownPath, summary); err != nil {
			fmt.Fprintln(os.Stderr, "ocreval:", err)
			os.Exit(1)
		}
	}

	if config.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(summary)
	} else {
		printSummary(summary)
	}

	if config.FailOnMismatch && (summary.Mismatched > 0 || summary.Errors > 0) {
		os.Exit(1)
	}
}

func parseFlags(args []string) (evalConfig, error) {
	var config evalConfig
	fs := flag.NewFlagSet("ocreval", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&config.ManifestPath, "manifest", "", "optional JSON sample manifest")
	fs.StringVar(&config.DirPath, "dir", "", "optional directory of captcha images")
	fs.BoolVar(&config.Recursive, "recursive", true, "scan sample directory recursively")
	fs.StringVar(&config.ExpectedFrom, "expected-from", "filename", "directory expected text source: filename or none")
	fs.StringVar(&config.Model, "model", envString("GODDDDOCR_MODEL", string(goddddocr.ModelOld)), "OCR model: old, beta, or custom")
	fs.StringVar(&config.ModelPath, "model-path", envString("GODDDDOCR_MODEL_PATH", ""), "path to a custom ONNX OCR model")
	fs.StringVar(&config.CharsetPath, "charset-path", envString("GODDDDOCR_CHARSET_PATH", ""), "path to a custom charset JSON array")
	fs.StringVar(&config.InputName, "input-name", envString("GODDDDOCR_INPUT_NAME", ""), "ONNX input name override")
	fs.StringVar(&config.OutputName, "output-name", envString("GODDDDOCR_OUTPUT_NAME", ""), "ONNX output name override")
	fs.StringVar(&config.SharedLibraryPath, "onnxruntime-lib", envString("ONNXRUNTIME_SHARED_LIBRARY_PATH", ""), "path to ONNX Runtime shared library")
	fs.BoolVar(&config.PNGFix, "png-fix", envBool("GODDDDOCR_PNG_FIX", false), "composite transparent PNGs over a white background")
	fs.StringVar(&config.CharsetRange, "charset-range", "", "optional charset range for all samples without a sample override")
	fs.StringVar(&config.ColorFilterColors, "color-filter-colors", "", "comma-separated color filter presets for all samples without a sample override")
	fs.BoolVar(&config.JSONOutput, "json", false, "print JSON summary")
	fs.StringVar(&config.CSVPath, "csv", "", "optional CSV output path")
	fs.StringVar(&config.MarkdownPath, "markdown", "", "optional Markdown output path")
	fs.BoolVar(&config.FailOnMismatch, "fail-on-mismatch", false, "exit non-zero when any sample mismatches or errors")
	if err := fs.Parse(args); err != nil {
		return evalConfig{}, err
	}
	return config, validateConfig(config)
}

func validateConfig(config evalConfig) error {
	if strings.TrimSpace(config.ManifestPath) == "" && strings.TrimSpace(config.DirPath) == "" {
		return fmt.Errorf("manifest or dir is required")
	}
	switch config.ExpectedFrom {
	case "filename", "none":
	default:
		return fmt.Errorf("expected-from must be filename or none")
	}
	return nil
}

func loadSamples(config evalConfig) ([]evalSample, error) {
	var samples []evalSample
	if strings.TrimSpace(config.ManifestPath) != "" {
		manifestSamples, err := loadManifest(config.ManifestPath)
		if err != nil {
			return nil, err
		}
		samples = append(samples, manifestSamples...)
	}
	if strings.TrimSpace(config.DirPath) != "" {
		dirSamples, err := scanDirectorySamples(config.DirPath, config.Recursive, config.ExpectedFrom)
		if err != nil {
			return nil, err
		}
		samples = append(samples, dirSamples...)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no samples found")
	}
	return samples, nil
}

func loadManifest(path string) ([]evalSample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var samples []evalSample
	if err := json.Unmarshal(data, &samples); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	for idx, sample := range samples {
		if sample.Name == "" {
			return nil, fmt.Errorf("manifest sample %d missing name", idx)
		}
		if sample.Image == "" {
			return nil, fmt.Errorf("manifest sample %q missing image", sample.Name)
		}
		if sample.Expected == "" && sample.PythonDDDDOCR != "" {
			samples[idx].Expected = sample.PythonDDDDOCR
		}
	}
	return samples, nil
}

func scanDirectorySamples(root string, recursive bool, expectedFrom string) ([]evalSample, error) {
	var samples []evalSample
	walkFn := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !isImagePath(path) {
			return nil
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		expected := ""
		if expectedFrom == "filename" {
			expected = name
		}
		samples = append(samples, evalSample{
			Name:     name,
			Image:    path,
			Expected: expected,
			Source:   "directory",
		})
		return nil
	}
	if err := filepath.WalkDir(root, walkFn); err != nil {
		return nil, fmt.Errorf("scan samples: %w", err)
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Image < samples[j].Image
	})
	return samples, nil
}

func isImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}

func runEval(config evalConfig, samples []evalSample) (evalSummary, error) {
	sessions := map[string]*goddddocr.OCR{}
	defer func() {
		for _, session := range sessions {
			_ = session.Close()
		}
	}()

	colors := splitCSV(config.ColorFilterColors)
	results := make([]evalResult, 0, len(samples))
	for _, sample := range samples {
		model := sample.Model
		if model == "" {
			model = goddddocr.Model(config.Model)
		}
		key := sessionKey(config, model)
		ocr := sessions[key]
		if ocr == nil {
			var err error
			ocr, err = goddddocr.NewOCR(goddddocr.Config{
				Model:             model,
				ModelPath:         config.ModelPath,
				CharsetPath:       config.CharsetPath,
				InputName:         config.InputName,
				OutputName:        config.OutputName,
				SharedLibraryPath: config.SharedLibraryPath,
				PNGFix:            config.PNGFix,
			})
			if err != nil {
				return evalSummary{}, err
			}
			sessions[key] = ocr
		}
		results = append(results, evalOne(ocr, sample, config, colors))
	}
	return summarize(results), nil
}

func evalOne(ocr *goddddocr.OCR, sample evalSample, config evalConfig, colors []string) evalResult {
	result := evalResult{
		Name:     sample.Name,
		Image:    sample.Image,
		Model:    string(ocr.Model()),
		Expected: sample.Expected,
		Labeled:  sample.Expected != "",
	}

	data, err := os.ReadFile(sample.Image)
	if err != nil {
		result.Error = fmt.Sprintf("read image: %v", err)
		return result
	}
	start := time.Now()
	ocrResult, err := ocr.ClassifyBytesDetailed(data, sampleOptions(sample, config, colors))
	result.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Result = ocrResult.Text
	result.Confidence = ocrResult.Confidence
	result.Matched = result.Labeled && ocrResult.Text == sample.Expected
	return result
}

func sampleOptions(sample evalSample, config evalConfig, colors []string) *goddddocr.ClassifyOptions {
	var options goddddocr.ClassifyOptions
	hasOptions := false
	if sample.PNGFix != nil {
		options.PNGFix = sample.PNGFix
		hasOptions = true
	}
	charsetRange := config.CharsetRange
	if sample.CharsetRange != "" {
		charsetRange = sample.CharsetRange
	}
	if charsetRange != "" {
		options.CharsetRange = goddddocr.NewCharsetRangeString(charsetRange)
		hasOptions = true
	}
	if len(sample.Colors) > 0 || len(sample.Ranges) > 0 {
		options.ColorFilter = &goddddocr.ColorFilterOptions{Colors: sample.Colors, Ranges: sample.Ranges}
		hasOptions = true
	} else if len(colors) > 0 {
		options.ColorFilter = &goddddocr.ColorFilterOptions{Colors: colors}
		hasOptions = true
	}
	if !hasOptions {
		return nil
	}
	return &options
}

func summarize(results []evalResult) evalSummary {
	summary := evalSummary{Total: len(results), Results: results}
	for _, result := range results {
		switch {
		case result.Error != "":
			summary.Errors++
		case !result.Labeled:
			summary.Unlabeled++
		case result.Matched:
			summary.Matched++
		default:
			summary.Mismatched++
		}
	}
	summary.Labeled = summary.Matched + summary.Mismatched
	if summary.Labeled > 0 {
		summary.Accuracy = float64(summary.Matched) / float64(summary.Labeled)
	}
	return summary
}

func writeCSV(path string, results []evalResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"name", "image", "model", "expected", "labeled", "result", "matched", "confidence", "elapsed_ms", "error"}); err != nil {
		return err
	}
	for _, result := range results {
		if err := writer.Write([]string{
			result.Name,
			result.Image,
			result.Model,
			result.Expected,
			fmt.Sprintf("%t", result.Labeled),
			result.Result,
			fmt.Sprintf("%t", result.Matched),
			fmt.Sprintf("%.6f", result.Confidence),
			fmt.Sprintf("%.3f", result.ElapsedMS),
			result.Error,
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeMarkdown(path string, summary evalSummary) error {
	return os.WriteFile(path, []byte(markdownSummary(summary)), 0o644)
}

func printSummary(summary evalSummary) {
	fmt.Print(markdownSummary(summary))
}

func markdownSummary(summary evalSummary) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# OCR Evaluation\n\n")
	fmt.Fprintf(&out, "- total: `%d`\n", summary.Total)
	fmt.Fprintf(&out, "- labeled: `%d`\n", summary.Labeled)
	fmt.Fprintf(&out, "- unlabeled: `%d`\n", summary.Unlabeled)
	fmt.Fprintf(&out, "- matched: `%d`\n", summary.Matched)
	fmt.Fprintf(&out, "- mismatched: `%d`\n", summary.Mismatched)
	fmt.Fprintf(&out, "- errors: `%d`\n", summary.Errors)
	fmt.Fprintf(&out, "- accuracy: `%.2f%%`\n\n", summary.Accuracy*100)
	fmt.Fprintf(&out, "| name | expected | result | labeled | matched | confidence | elapsed ms | error |\n")
	fmt.Fprintf(&out, "|---|---|---|---:|---:|---:|---:|---|\n")
	for _, result := range summary.Results {
		fmt.Fprintf(&out, "| %s | %s | %s | %t | %t | %.4f | %.3f | %s |\n",
			escapeMarkdown(result.Name),
			escapeMarkdown(result.Expected),
			escapeMarkdown(result.Result),
			result.Labeled,
			result.Matched,
			result.Confidence,
			result.ElapsedMS,
			escapeMarkdown(result.Error),
		)
	}
	return out.String()
}

func escapeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func sessionKey(config evalConfig, model goddddocr.Model) string {
	return strings.Join([]string{
		string(model),
		config.ModelPath,
		config.CharsetPath,
		config.InputName,
		config.OutputName,
		config.SharedLibraryPath,
	}, "\x00")
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
