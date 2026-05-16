package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strings"

	"github.com/tensafe/goddddocr"
)

type prepConfig struct {
	ImagePath         string
	OutputPath        string
	JSONPath          string
	MatrixCSVPath     string
	PNGFix            bool
	ColorFilterColors string
	ColorFilterRanges string
}

type prepReport struct {
	ImagePath     string   `json:"image_path"`
	OutputPath    string   `json:"output_path,omitempty"`
	MatrixCSVPath string   `json:"matrix_csv_path,omitempty"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Pixels        int      `json:"pixels"`
	Min           float64  `json:"min"`
	Max           float64  `json:"max"`
	Mean          float64  `json:"mean"`
	SHA256        string   `json:"sha256"`
	PNGFix        bool     `json:"png_fix"`
	Colors        []string `json:"color_filter_colors,omitempty"`
	RangeCount    int      `json:"color_filter_range_count,omitempty"`
}

func main() {
	config, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocrprep:", err)
		os.Exit(2)
	}

	report, err := run(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocrprep:", err)
		os.Exit(1)
	}

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocrprep:", err)
		os.Exit(1)
	}
	if config.JSONPath != "" {
		if err := os.WriteFile(config.JSONPath, append(encoded, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "ocrprep:", err)
			os.Exit(1)
		}
	}
	fmt.Println(string(encoded))
}

func parseFlags(args []string) (prepConfig, error) {
	var config prepConfig
	fs := flag.NewFlagSet("ocrprep", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&config.ImagePath, "image", "", "image file to preprocess")
	fs.StringVar(&config.OutputPath, "out", "", "optional grayscale PNG output path")
	fs.StringVar(&config.JSONPath, "json", "", "optional JSON report output path")
	fs.StringVar(&config.MatrixCSVPath, "matrix-csv", "", "optional grayscale pixel matrix CSV output path")
	fs.BoolVar(&config.PNGFix, "png-fix", false, "composite transparent PNGs over a white background")
	fs.StringVar(&config.ColorFilterColors, "color-filter-colors", "", "comma-separated color filter presets")
	fs.StringVar(&config.ColorFilterRanges, "color-filter-custom-ranges", "", "JSON custom HSV ranges")
	if err := fs.Parse(args); err != nil {
		return prepConfig{}, err
	}
	if strings.TrimSpace(config.ImagePath) == "" {
		return prepConfig{}, fmt.Errorf("image is required")
	}
	return config, nil
}

func run(config prepConfig) (prepReport, error) {
	data, err := os.ReadFile(config.ImagePath)
	if err != nil {
		return prepReport{}, fmt.Errorf("read image: %w", err)
	}
	colors := splitCSV(config.ColorFilterColors)
	ranges, err := parseHSVRanges(config.ColorFilterRanges)
	if err != nil {
		return prepReport{}, err
	}

	var colorFilter *goddddocr.ColorFilterOptions
	if len(colors) > 0 || len(ranges) > 0 {
		colorFilter = &goddddocr.ColorFilterOptions{Colors: colors, Ranges: ranges}
	}
	result, err := goddddocr.PreprocessOCRBytes(data, &goddddocr.PreprocessOptions{
		PNGFix:      config.PNGFix,
		ColorFilter: colorFilter,
	})
	if err != nil {
		return prepReport{}, err
	}
	gray, err := result.GrayImage()
	if err != nil {
		return prepReport{}, err
	}

	if config.OutputPath != "" {
		if err := writePNG(config.OutputPath, gray); err != nil {
			return prepReport{}, err
		}
	}
	if config.MatrixCSVPath != "" {
		if err := writeMatrixCSV(config.MatrixCSVPath, gray.Pix, result.Width, result.Height); err != nil {
			return prepReport{}, err
		}
	}

	report := summarize(result, gray.Pix)
	report.ImagePath = config.ImagePath
	report.OutputPath = config.OutputPath
	report.MatrixCSVPath = config.MatrixCSVPath
	report.PNGFix = config.PNGFix
	report.Colors = colors
	report.RangeCount = len(ranges)
	return report, nil
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create png: %w", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}

func writeMatrixCSV(path string, pixels []uint8, width int, height int) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create matrix csv: %w", err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	for y := 0; y < height; y++ {
		row := make([]string, width)
		for x := 0; x < width; x++ {
			row[x] = fmt.Sprintf("%d", pixels[y*width+x])
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}

func summarize(result *goddddocr.PreprocessResult, pixels []uint8) prepReport {
	hash := sha256.Sum256(pixels)
	report := prepReport{
		Width:  result.Width,
		Height: result.Height,
		Pixels: len(pixels),
		SHA256: hex.EncodeToString(hash[:]),
	}
	if len(pixels) == 0 {
		return report
	}
	minValue := uint8(255)
	maxValue := uint8(0)
	var sum float64
	for _, value := range pixels {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
		sum += float64(value)
	}
	report.Min = float64(minValue)
	report.Max = float64(maxValue)
	report.Mean = math.Round((sum/float64(len(pixels)))*1000) / 1000
	return report
}

func parseHSVRanges(value string) ([]goddddocr.HSVRange, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	var ranges []goddddocr.HSVRange
	if err := json.Unmarshal([]byte(value), &ranges); err == nil {
		return ranges, nil
	}
	var single goddddocr.HSVRange
	if err := json.Unmarshal([]byte(value), &single); err != nil {
		return nil, fmt.Errorf("decode color-filter-custom-ranges: %w", err)
	}
	return []goddddocr.HSVRange{single}, nil
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
