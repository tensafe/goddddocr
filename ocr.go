package goddddocr

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type Model string

const (
	ModelOld  Model = "old"
	ModelBeta Model = "beta"
)

type Config struct {
	Model             Model
	SharedLibraryPath string
	PNGFix            bool
}

type OCREngine interface {
	Model() Model
	ClassifyBytesDetailed(data []byte, options *ClassifyOptions) (*ClassifyResult, error)
}

type CharsetRange struct {
	limit *int
	chars []string
}

func NewCharsetRangeLimit(maxIndex int) *CharsetRange {
	return &CharsetRange{limit: &maxIndex}
}

func NewCharsetRangeString(chars string) *CharsetRange {
	out := make([]string, 0, len(chars))
	seen := map[string]struct{}{}
	for _, r := range chars {
		ch := string(r)
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		out = append(out, ch)
	}
	return &CharsetRange{chars: out}
}

func NewCharsetRangeChars(chars []string) *CharsetRange {
	out := make([]string, 0, len(chars))
	seen := map[string]struct{}{}
	for _, ch := range chars {
		if ch == "" {
			continue
		}
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		out = append(out, ch)
	}
	return &CharsetRange{chars: out}
}

type ClassifyOptions struct {
	PNGFix       *bool
	CharsetRange *CharsetRange
	Probability  bool
}

type ClassifyResult struct {
	Text        string             `json:"text"`
	Confidence  float64            `json:"confidence"`
	Probability *ProbabilityMatrix `json:"probability,omitempty"`
}

type ProbabilityMatrix struct {
	Text        string      `json:"text"`
	Charsets    []string    `json:"charsets"`
	Probability [][]float64 `json:"probability"`
	Confidence  float64     `json:"confidence"`
}

type OCR struct {
	model  Model
	chars  []string
	pngFix bool

	session *ort.DynamicAdvancedSession
	mu      sync.Mutex
}

func NewOCR(config Config) (*OCR, error) {
	model := config.Model
	if model == "" {
		model = ModelOld
	}
	if model != ModelOld && model != ModelBeta {
		return nil, fmt.Errorf("unsupported model %q", model)
	}

	if err := InitRuntime(config.SharedLibraryPath); err != nil {
		return nil, err
	}

	modelPath := "assets/models/common_old.onnx"
	if model == ModelBeta {
		modelPath = "assets/models/common.onnx"
	}
	modelData, err := embeddedFiles.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("read model %q: %w", modelPath, err)
	}

	session, err := ort.NewDynamicAdvancedSessionWithONNXData(
		modelData,
		[]string{"input1"},
		[]string{"387"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	chars, err := loadCharset(model)
	if err != nil {
		_ = session.Destroy()
		return nil, err
	}

	return &OCR{
		model:   model,
		chars:   chars,
		pngFix:  config.PNGFix,
		session: session,
	}, nil
}

func (o *OCR) Close() error {
	if o == nil || o.session == nil {
		return nil
	}
	return o.session.Destroy()
}

func (o *OCR) Model() Model {
	return o.model
}

func (o *OCR) Charset() []string {
	out := make([]string, len(o.chars))
	copy(out, o.chars)
	return out
}

func (o *OCR) ClassifyBytes(data []byte, options *ClassifyOptions) (string, error) {
	result, err := o.ClassifyBytesDetailed(data, options)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (o *OCR) ClassifyBytesDetailed(data []byte, options *ClassifyOptions) (*ClassifyResult, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return o.ClassifyImageDetailed(img, options)
}

func (o *OCR) ClassifyImage(img image.Image, options *ClassifyOptions) (string, error) {
	result, err := o.ClassifyImageDetailed(img, options)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (o *OCR) ClassifyImageDetailed(img image.Image, options *ClassifyOptions) (*ClassifyResult, error) {
	if o == nil || o.session == nil {
		return nil, fmt.Errorf("OCR engine is closed")
	}

	pngFix := o.pngFix
	if options != nil && options.PNGFix != nil {
		pngFix = *options.PNGFix
	}

	inputData, width, err := preprocessOCRImage(img, pngFix)
	if err != nil {
		return nil, err
	}

	inputTensor, err := ort.NewTensor(ort.NewShape(1, 1, ocrTargetHeight, int64(width)), inputData)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputs := []ort.Value{nil}
	o.mu.Lock()
	err = o.session.Run([]ort.Value{inputTensor}, outputs)
	o.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("run OCR model: %w", err)
	}
	if outputs[0] == nil {
		return nil, fmt.Errorf("OCR model returned no output")
	}
	defer outputs[0].Destroy()

	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected OCR output tensor type %T", outputs[0])
	}

	valid, err := o.validIndices(options)
	if err != nil {
		return nil, err
	}
	includeProbability := options != nil && options.Probability
	return processOCROutput(outputTensor.GetData(), outputTensor.GetShape(), o.chars, valid, includeProbability)
}

func (o *OCR) validIndices(options *ClassifyOptions) (map[int]struct{}, error) {
	if options == nil || options.CharsetRange == nil {
		return nil, nil
	}
	spec := options.CharsetRange
	valid := map[int]struct{}{}
	if spec.limit != nil {
		if *spec.limit < 0 {
			return nil, fmt.Errorf("charset range limit must be non-negative")
		}
		max := *spec.limit
		if max >= len(o.chars) {
			max = len(o.chars) - 1
		}
		for idx := 0; idx <= max; idx++ {
			valid[idx] = struct{}{}
		}
		valid[0] = struct{}{}
		return valid, nil
	}

	indexByChar := make(map[string]int, len(o.chars))
	for idx, ch := range o.chars {
		indexByChar[ch] = idx
	}
	for _, ch := range spec.chars {
		if idx, ok := indexByChar[ch]; ok {
			valid[idx] = struct{}{}
		}
	}
	valid[0] = struct{}{}
	return valid, nil
}

func processOCROutput(data []float32, shape ort.Shape, charset []string, valid map[int]struct{}, includeProbability bool) (*ClassifyResult, error) {
	rows, err := ocrOutputRows(data, shape)
	if err != nil {
		return nil, err
	}

	indices := make([]int, len(rows))
	confidences := make([]float64, len(rows))
	var probabilities [][]float64
	if includeProbability {
		probabilities = make([][]float64, len(rows))
	}

	for idx, row := range rows {
		indices[idx] = argmax(row)
		if includeProbability {
			probabilities[idx] = softmax(row)
			confidences[idx] = maxFloat64(probabilities[idx])
		} else {
			confidences[idx] = maxSoftmax(row)
		}
	}

	var text strings.Builder
	prev := -1
	for _, idx := range indices {
		if _, restricted := valid[idx]; valid != nil && !restricted {
			prev = idx
			continue
		}
		if idx != prev && idx != 0 && idx >= 0 && idx < len(charset) {
			text.WriteString(charset[idx])
		}
		prev = idx
	}

	result := &ClassifyResult{
		Text:       text.String(),
		Confidence: mean(confidences),
	}
	if includeProbability {
		charsets := make([]string, len(charset))
		copy(charsets, charset)
		result.Probability = &ProbabilityMatrix{
			Text:        result.Text,
			Charsets:    charsets,
			Probability: probabilities,
			Confidence:  result.Confidence,
		}
	}
	return result, nil
}

func ocrOutputRows(data []float32, shape ort.Shape) ([][]float32, error) {
	if len(shape) == 0 {
		return nil, fmt.Errorf("empty OCR output shape")
	}

	switch len(shape) {
	case 3:
		d0, d1, d2 := int(shape[0]), int(shape[1]), int(shape[2])
		if d0 <= 0 || d1 <= 0 || d2 <= 0 {
			return nil, fmt.Errorf("invalid OCR output shape %v", shape)
		}
		if len(data) < d0*d1*d2 {
			return nil, fmt.Errorf("OCR output data shorter than shape %v", shape)
		}
		var rows [][]float32
		if d1 == 1 {
			rows = make([][]float32, d0)
			for t := 0; t < d0; t++ {
				row := data[t*d1*d2 : t*d1*d2+d2]
				rows[t] = row
			}
		} else if d0 == 1 {
			rows = make([][]float32, d1)
			for t := 0; t < d1; t++ {
				offset := t * d2
				row := data[offset : offset+d2]
				rows[t] = row
			}
		} else {
			rows = make([][]float32, d1)
			for t := 0; t < d1; t++ {
				offset := t * d2
				row := data[offset : offset+d2]
				rows[t] = row
			}
		}
		return rows, nil
	case 2:
		rows, cols := int(shape[0]), int(shape[1])
		if rows <= 0 || cols <= 0 {
			return nil, fmt.Errorf("invalid OCR output shape %v", shape)
		}
		if len(data) < rows*cols {
			return nil, fmt.Errorf("OCR output data shorter than shape %v", shape)
		}
		out := make([][]float32, rows)
		for row := 0; row < rows; row++ {
			offset := row * cols
			out[row] = data[offset : offset+cols]
		}
		return out, nil
	default:
		if len(data) == 0 {
			return nil, fmt.Errorf("empty OCR output data")
		}
		return [][]float32{data}, nil
	}
}

func argmax(values []float32) int {
	if len(values) == 0 {
		return -1
	}
	bestIndex := 0
	bestValue := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] > bestValue {
			bestIndex = i
			bestValue = values[i]
		}
	}
	return bestIndex
}

func maxSoftmax(values []float32) float64 {
	if len(values) == 0 {
		return 0
	}
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	var sum float64
	for _, value := range values {
		sum += math.Exp(float64(value - maxValue))
	}
	if sum == 0 {
		return 0
	}
	return 1 / sum
}

func softmax(values []float32) []float64 {
	if len(values) == 0 {
		return nil
	}
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}

	out := make([]float64, len(values))
	var sum float64
	for idx, value := range values {
		probability := math.Exp(float64(value - maxValue))
		out[idx] = probability
		sum += probability
	}
	if sum == 0 {
		return out
	}
	for idx := range out {
		out[idx] /= sum
	}
	return out
}

func maxFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	best := values[0]
	for _, value := range values[1:] {
		if value > best {
			best = value
		}
	}
	return best
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
