package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/tensafe/goddddocr"
)

type prepConfig struct {
	ImagePath         string
	OutputPath        string
	JSONPath          string
	MatrixCSVPath     string
	ComparePNGPath    string
	CompareCSVPath    string
	DiffPNGPath       string
	PNGFix            bool
	ColorFilterColors string
	ColorFilterRanges string
}

type prepReport struct {
	ImagePath     string      `json:"image_path"`
	OutputPath    string      `json:"output_path,omitempty"`
	MatrixCSVPath string      `json:"matrix_csv_path,omitempty"`
	ComparePNG    string      `json:"compare_png,omitempty"`
	CompareCSV    string      `json:"compare_csv,omitempty"`
	DiffPNGPath   string      `json:"diff_png_path,omitempty"`
	Width         int         `json:"width"`
	Height        int         `json:"height"`
	Pixels        int         `json:"pixels"`
	Min           float64     `json:"min"`
	Max           float64     `json:"max"`
	Mean          float64     `json:"mean"`
	SHA256        string      `json:"sha256"`
	PNGFix        bool        `json:"png_fix"`
	Colors        []string    `json:"color_filter_colors,omitempty"`
	RangeCount    int         `json:"color_filter_range_count,omitempty"`
	Diff          *diffReport `json:"diff,omitempty"`
}

type diffReport struct {
	ReferenceSHA256    string      `json:"reference_sha256"`
	ExactMatch         bool        `json:"exact_match"`
	DifferentPixels    int         `json:"different_pixels"`
	DifferentPixelRate float64     `json:"different_pixel_rate"`
	MaxAbsDiff         int         `json:"max_abs_diff"`
	MeanAbsDiff        float64     `json:"mean_abs_diff"`
	RMSE               float64     `json:"rmse"`
	FirstDifferences   []pixelDiff `json:"first_differences,omitempty"`
}

type pixelDiff struct {
	Index     int `json:"index"`
	X         int `json:"x"`
	Y         int `json:"y"`
	Actual    int `json:"actual"`
	Reference int `json:"reference"`
	Delta     int `json:"delta"`
	AbsDiff   int `json:"abs_diff"`
}

const maxDiffSamples = 20

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
	fs.StringVar(&config.ComparePNGPath, "compare-png", "", "optional reference grayscale PNG to compare")
	fs.StringVar(&config.CompareCSVPath, "compare-csv", "", "optional reference pixel matrix CSV to compare")
	fs.StringVar(&config.DiffPNGPath, "diff-png", "", "optional visual diff PNG output path; requires compare-png or compare-csv")
	fs.BoolVar(&config.PNGFix, "png-fix", false, "composite transparent PNGs over a white background")
	fs.StringVar(&config.ColorFilterColors, "color-filter-colors", "", "comma-separated color filter presets")
	fs.StringVar(&config.ColorFilterRanges, "color-filter-custom-ranges", "", "JSON custom HSV ranges")
	if err := fs.Parse(args); err != nil {
		return prepConfig{}, err
	}
	if strings.TrimSpace(config.ImagePath) == "" {
		return prepConfig{}, fmt.Errorf("image is required")
	}
	if config.DiffPNGPath != "" && config.ComparePNGPath == "" && config.CompareCSVPath == "" {
		return prepConfig{}, fmt.Errorf("diff-png requires compare-png or compare-csv")
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
	report.ComparePNG = config.ComparePNGPath
	report.CompareCSV = config.CompareCSVPath
	report.DiffPNGPath = config.DiffPNGPath
	report.PNGFix = config.PNGFix
	report.Colors = colors
	report.RangeCount = len(ranges)
	if config.ComparePNGPath != "" || config.CompareCSVPath != "" {
		reference, referenceWidth, referenceHeight, err := readReference(config)
		if err != nil {
			return prepReport{}, err
		}
		if referenceWidth != result.Width || referenceHeight != result.Height {
			return prepReport{}, fmt.Errorf("reference dimensions %dx%d do not match Go preprocessing %dx%d", referenceWidth, referenceHeight, result.Width, result.Height)
		}
		diff, err := comparePixels(gray.Pix, reference, result.Width)
		if err != nil {
			return prepReport{}, err
		}
		if config.DiffPNGPath != "" {
			if err := writeDiffPNG(config.DiffPNGPath, gray.Pix, reference, result.Width, result.Height); err != nil {
				return prepReport{}, err
			}
		}
		report.Diff = diff
	}
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

func compareReference(config prepConfig, pixels []uint8, width int, height int) (*diffReport, error) {
	reference, referenceWidth, referenceHeight, err := readReference(config)
	if err != nil {
		return nil, err
	}
	if referenceWidth != width || referenceHeight != height {
		return nil, fmt.Errorf("reference dimensions %dx%d do not match Go preprocessing %dx%d", referenceWidth, referenceHeight, width, height)
	}
	return comparePixels(pixels, reference, width)
}

func readReference(config prepConfig) ([]uint8, int, int, error) {
	if config.ComparePNGPath != "" && config.CompareCSVPath != "" {
		return nil, 0, 0, fmt.Errorf("only one of compare-png or compare-csv may be set")
	}
	if config.ComparePNGPath != "" {
		return readGrayPNG(config.ComparePNGPath)
	}
	if config.CompareCSVPath != "" {
		return readMatrixCSV(config.CompareCSVPath)
	}
	return nil, 0, 0, fmt.Errorf("compare-png or compare-csv is required")
}

func readGrayPNG(path string) ([]uint8, int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("open reference png: %w", err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode reference png: %w", err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := make([]uint8, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			gray := color.GrayModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
			pixels = append(pixels, gray.Y)
		}
	}
	return pixels, width, height, nil
}

func readMatrixCSV(path string) ([]uint8, int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("open reference csv: %w", err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("read reference csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, 0, 0, fmt.Errorf("reference csv is empty")
	}
	width := len(rows[0])
	if width == 0 {
		return nil, 0, 0, fmt.Errorf("reference csv first row is empty")
	}
	pixels := make([]uint8, 0, width*len(rows))
	for rowIdx, row := range rows {
		if len(row) != width {
			return nil, 0, 0, fmt.Errorf("reference csv row %d width %d does not match first row width %d", rowIdx+1, len(row), width)
		}
		for colIdx, cell := range row {
			value, err := strconv.Atoi(strings.TrimSpace(cell))
			if err != nil || value < 0 || value > 255 {
				return nil, 0, 0, fmt.Errorf("reference csv cell %d,%d must be 0..255", rowIdx+1, colIdx+1)
			}
			pixels = append(pixels, uint8(value))
		}
	}
	return pixels, width, len(rows), nil
}

func comparePixels(actual []uint8, reference []uint8, width int) (*diffReport, error) {
	if len(actual) != len(reference) {
		return nil, fmt.Errorf("reference pixel count %d does not match actual %d", len(reference), len(actual))
	}
	if len(actual) > 0 && width <= 0 {
		return nil, fmt.Errorf("width must be positive")
	}
	hash := sha256.Sum256(reference)
	report := &diffReport{
		ReferenceSHA256: hex.EncodeToString(hash[:]),
		ExactMatch:      true,
	}
	if len(actual) == 0 {
		return report, nil
	}
	var totalAbsDiff float64
	var totalSquaredDiff float64
	for idx := range actual {
		delta := int(actual[idx]) - int(reference[idx])
		absDiff := delta
		if absDiff < 0 {
			absDiff = -absDiff
		}
		if absDiff > 0 {
			report.ExactMatch = false
			report.DifferentPixels++
			if len(report.FirstDifferences) < maxDiffSamples {
				report.FirstDifferences = append(report.FirstDifferences, pixelDiff{
					Index:     idx,
					X:         idx % width,
					Y:         idx / width,
					Actual:    int(actual[idx]),
					Reference: int(reference[idx]),
					Delta:     delta,
					AbsDiff:   absDiff,
				})
			}
		}
		if absDiff > report.MaxAbsDiff {
			report.MaxAbsDiff = absDiff
		}
		totalAbsDiff += float64(absDiff)
		totalSquaredDiff += float64(absDiff * absDiff)
	}
	report.DifferentPixelRate = roundFloat(float64(report.DifferentPixels)/float64(len(actual)), 6)
	report.MeanAbsDiff = roundFloat(totalAbsDiff/float64(len(actual)), 6)
	report.RMSE = roundFloat(math.Sqrt(totalSquaredDiff/float64(len(actual))), 6)
	return report, nil
}

func writeDiffPNG(path string, actual []uint8, reference []uint8, width int, height int) error {
	if len(actual) != len(reference) {
		return fmt.Errorf("reference pixel count %d does not match actual %d", len(reference), len(actual))
	}
	if width < 0 || height < 0 || len(actual) != width*height {
		return fmt.Errorf("pixel count %d does not match dimensions %dx%d", len(actual), width, height)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			delta := int(actual[idx]) - int(reference[idx])
			if delta == 0 {
				img.SetRGBA(x, y, color.RGBA{A: 255})
				continue
			}
			intensity := diffIntensity(delta)
			if delta < 0 {
				img.SetRGBA(x, y, color.RGBA{R: intensity, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{B: intensity, A: 255})
			}
		}
	}
	return writePNG(path, img)
}

func diffIntensity(delta int) uint8 {
	if delta < 0 {
		delta = -delta
	}
	intensity := delta * 8
	if intensity < 32 {
		intensity = 32
	}
	if intensity > 255 {
		intensity = 255
	}
	return uint8(intensity)
}

func roundFloat(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
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
