package goddddocr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sort"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	defaultDetectionInputName  = "images"
	defaultDetectionOutputName = "output"
	defaultDetectionInputSize  = 416
	defaultDetectionScore      = 0.1
	defaultDetectionNMS        = 0.45
)

type DetectionConfig struct {
	ModelPath         string
	InputName         string
	OutputName        string
	SharedLibraryPath string
	InputSize         int
	ScoreThreshold    float64
	NMSThreshold      float64
}

type DetectionOptions struct {
	ScoreThreshold *float64 `json:"score_threshold,omitempty"`
	NMSThreshold   *float64 `json:"nms_threshold,omitempty"`
}

type DetectionBox struct {
	X1      int     `json:"x1"`
	Y1      int     `json:"y1"`
	X2      int     `json:"x2"`
	Y2      int     `json:"y2"`
	Score   float64 `json:"score,omitempty"`
	ClassID int     `json:"class_id,omitempty"`
}

func (b DetectionBox) Rect() []int {
	return []int{b.X1, b.Y1, b.X2, b.Y2}
}

type DetectionEngine interface {
	DetectBytesDetailed(data []byte) ([]DetectionBox, error)
}

type Detector struct {
	inputSize      int
	scoreThreshold float64
	nmsThreshold   float64
	session        *ort.DynamicAdvancedSession
	mu             sync.Mutex
}

func NewDetector(config DetectionConfig) (*Detector, error) {
	if err := InitRuntime(config.SharedLibraryPath); err != nil {
		return nil, err
	}

	modelData, modelSource, err := loadDetectionModelData(config.ModelPath)
	if err != nil {
		return nil, err
	}

	inputName := strings.TrimSpace(config.InputName)
	if inputName == "" {
		inputName = defaultDetectionInputName
	}
	outputName := strings.TrimSpace(config.OutputName)
	if outputName == "" {
		outputName = defaultDetectionOutputName
	}

	session, err := ort.NewDynamicAdvancedSessionWithONNXData(
		modelData,
		[]string{inputName},
		[]string{outputName},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create detection ONNX session from %s: %w", modelSource, err)
	}

	inputSize := config.InputSize
	if inputSize == 0 {
		inputSize = defaultDetectionInputSize
	}
	if inputSize <= 0 {
		_ = session.Destroy()
		return nil, fmt.Errorf("detection input size must be positive")
	}

	scoreThreshold := config.ScoreThreshold
	if scoreThreshold == 0 {
		scoreThreshold = defaultDetectionScore
	}
	nmsThreshold := config.NMSThreshold
	if nmsThreshold == 0 {
		nmsThreshold = defaultDetectionNMS
	}

	return &Detector{
		inputSize:      inputSize,
		scoreThreshold: scoreThreshold,
		nmsThreshold:   nmsThreshold,
		session:        session,
	}, nil
}

func (d *Detector) Close() error {
	if d == nil || d.session == nil {
		return nil
	}
	return d.session.Destroy()
}

func (d *Detector) DetectBytes(data []byte) ([][]int, error) {
	boxes, err := d.DetectBytesDetailed(data)
	if err != nil {
		return nil, err
	}
	out := make([][]int, len(boxes))
	for idx, box := range boxes {
		out[idx] = box.Rect()
	}
	return out, nil
}

func (d *Detector) DetectBytesDetailed(data []byte) ([]DetectionBox, error) {
	return d.DetectBytesDetailedWithOptions(data, nil)
}

func (d *Detector) DetectBytesDetailedWithOptions(data []byte, options *DetectionOptions) ([]DetectionBox, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return d.DetectImageDetailedWithOptions(img, options)
}

func (d *Detector) DetectImageDetailed(img image.Image) ([]DetectionBox, error) {
	return d.DetectImageDetailedWithOptions(img, nil)
}

func (d *Detector) DetectImageDetailedWithOptions(img image.Image, options *DetectionOptions) ([]DetectionBox, error) {
	if d == nil || d.session == nil {
		return nil, fmt.Errorf("detection engine is closed")
	}
	scoreThreshold, nmsThreshold, err := d.thresholds(options)
	if err != nil {
		return nil, err
	}

	inputData, ratio, width, height, err := preprocessDetectionImage(img, d.inputSize)
	if err != nil {
		return nil, err
	}

	inputTensor, err := ort.NewTensor(ort.NewShape(1, 3, int64(d.inputSize), int64(d.inputSize)), inputData)
	if err != nil {
		return nil, fmt.Errorf("create detection input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputs := []ort.Value{nil}
	d.mu.Lock()
	err = d.session.Run([]ort.Value{inputTensor}, outputs)
	d.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("run detection model: %w", err)
	}
	if outputs[0] == nil {
		return nil, fmt.Errorf("detection model returned no output")
	}
	defer outputs[0].Destroy()

	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected detection output tensor type %T", outputs[0])
	}
	return processDetectionOutput(outputTensor.GetData(), outputTensor.GetShape(), ratio, width, height, d.inputSize, scoreThreshold, nmsThreshold)
}

func (d *Detector) thresholds(options *DetectionOptions) (float64, float64, error) {
	scoreThreshold := d.scoreThreshold
	nmsThreshold := d.nmsThreshold
	if options != nil {
		if options.ScoreThreshold != nil {
			scoreThreshold = *options.ScoreThreshold
		}
		if options.NMSThreshold != nil {
			nmsThreshold = *options.NMSThreshold
		}
	}
	if scoreThreshold < 0 || scoreThreshold > 1 {
		return 0, 0, fmt.Errorf("score_threshold must be between 0 and 1")
	}
	if nmsThreshold < 0 || nmsThreshold > 1 {
		return 0, 0, fmt.Errorf("nms_threshold must be between 0 and 1")
	}
	return scoreThreshold, nmsThreshold, nil
}

func loadDetectionModelData(customPath string) ([]byte, string, error) {
	if strings.TrimSpace(customPath) != "" {
		data, err := os.ReadFile(customPath)
		if err != nil {
			return nil, "", fmt.Errorf("read custom detection model %q: %w", customPath, err)
		}
		if len(data) == 0 {
			return nil, "", fmt.Errorf("custom detection model %q is empty", customPath)
		}
		return data, customPath, nil
	}

	const path = "assets/models/common_det.onnx"
	data, err := embeddedFiles.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read detection model %q: %w", path, err)
	}
	return data, path, nil
}

func preprocessDetectionImage(img image.Image, inputSize int) ([]float32, float64, int, int, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, 0, 0, 0, fmt.Errorf("empty image")
	}
	if inputSize <= 0 {
		return nil, 0, 0, 0, fmt.Errorf("detection input size must be positive")
	}

	ratio := math.Min(float64(inputSize)/float64(height), float64(inputSize)/float64(width))
	resizedWidth := int(float64(width) * ratio)
	resizedHeight := int(float64(height) * ratio)
	if resizedWidth <= 0 || resizedHeight <= 0 {
		return nil, 0, 0, 0, fmt.Errorf("invalid resized detection dimensions %dx%d", resizedWidth, resizedHeight)
	}

	resized := resizeOpenCVLinear(img, resizedWidth, resizedHeight)
	plane := inputSize * inputSize
	data := make([]float32, 3*plane)
	for idx := range data {
		data[idx] = 114
	}
	for y := 0; y < resizedHeight; y++ {
		for x := 0; x < resizedWidth; x++ {
			c := resized[y*resizedWidth+x]
			offset := y*inputSize + x
			data[offset] = float32(c.B)
			data[plane+offset] = float32(c.G)
			data[2*plane+offset] = float32(c.R)
		}
	}
	return data, ratio, width, height, nil
}

func resizeOpenCVLinear(img image.Image, dstWidth int, dstHeight int) []color.NRGBA {
	bounds := img.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	out := make([]color.NRGBA, dstWidth*dstHeight)
	if srcWidth <= 0 || srcHeight <= 0 || dstWidth <= 0 || dstHeight <= 0 {
		return out
	}

	xOfs, xAlpha := linearResizeCoefficients(srcWidth, dstWidth)
	yOfs, yAlpha := linearResizeCoefficients(srcHeight, dstHeight)
	for y := 0; y < dstHeight; y++ {
		sy := yOfs[y]
		ay := yAlpha[y]
		for x := 0; x < dstWidth; x++ {
			sx := xOfs[x]
			ax := xAlpha[x]
			c00 := color.NRGBAModel.Convert(img.At(bounds.Min.X+sx, bounds.Min.Y+sy)).(color.NRGBA)
			c01 := color.NRGBAModel.Convert(img.At(bounds.Min.X+minInt(sx+1, srcWidth-1), bounds.Min.Y+sy)).(color.NRGBA)
			c10 := color.NRGBAModel.Convert(img.At(bounds.Min.X+sx, bounds.Min.Y+minInt(sy+1, srcHeight-1))).(color.NRGBA)
			c11 := color.NRGBAModel.Convert(img.At(bounds.Min.X+minInt(sx+1, srcWidth-1), bounds.Min.Y+minInt(sy+1, srcHeight-1))).(color.NRGBA)
			out[y*dstWidth+x] = color.NRGBA{
				R: interpolateUint8(c00.R, c01.R, c10.R, c11.R, ax, ay),
				G: interpolateUint8(c00.G, c01.G, c10.G, c11.G, ax, ay),
				B: interpolateUint8(c00.B, c01.B, c10.B, c11.B, ax, ay),
				A: interpolateUint8(c00.A, c01.A, c10.A, c11.A, ax, ay),
			}
		}
	}
	return out
}

func linearResizeCoefficients(srcSize int, dstSize int) ([]int, []float64) {
	ofs := make([]int, dstSize)
	alpha := make([]float64, dstSize)
	if srcSize <= 1 {
		return ofs, alpha
	}
	scale := float64(srcSize) / float64(dstSize)
	for dst := 0; dst < dstSize; dst++ {
		src := (float64(dst)+0.5)*scale - 0.5
		idx := int(math.Floor(src))
		weight := src - float64(idx)
		if idx < 0 {
			idx = 0
			weight = 0
		}
		if idx >= srcSize-1 {
			idx = srcSize - 2
			weight = 1
		}
		ofs[dst] = idx
		alpha[dst] = weight
	}
	return ofs, alpha
}

func interpolateUint8(c00 uint8, c01 uint8, c10 uint8, c11 uint8, ax float64, ay float64) uint8 {
	top := float64(c00)*(1-ax) + float64(c01)*ax
	bottom := float64(c10)*(1-ax) + float64(c11)*ax
	value := top*(1-ay) + bottom*ay
	if value <= 0 {
		return 0
	}
	if value >= 255 {
		return 255
	}
	return uint8(math.Round(value))
}

type detectionCandidate struct {
	x1      float64
	y1      float64
	x2      float64
	y2      float64
	score   float64
	classID int
}

func processDetectionOutput(data []float32, shape ort.Shape, ratio float64, imageWidth int, imageHeight int, inputSize int, scoreThreshold float64, nmsThreshold float64) ([]DetectionBox, error) {
	rows, cols, err := detectionRows(data, shape)
	if err != nil {
		return nil, err
	}
	if ratio <= 0 {
		return nil, fmt.Errorf("detection ratio must be positive")
	}
	if inputSize <= 0 {
		return nil, fmt.Errorf("detection input size must be positive")
	}

	grids := detectionGrids(inputSize)
	if rows > len(grids) {
		return nil, fmt.Errorf("detection output rows %d exceed grid count %d", rows, len(grids))
	}

	candidates := make([]detectionCandidate, 0)
	for rowIdx := 0; rowIdx < rows; rowIdx++ {
		row := data[rowIdx*cols : (rowIdx+1)*cols]
		grid := grids[rowIdx]
		cx := (float64(row[0]) + float64(grid.x)) * float64(grid.stride)
		cy := (float64(row[1]) + float64(grid.y)) * float64(grid.stride)
		w := math.Exp(float64(row[2])) * float64(grid.stride)
		h := math.Exp(float64(row[3])) * float64(grid.stride)
		obj := float64(row[4])
		bestScore := 0.0
		bestClass := 0
		for classIdx, raw := range row[5:] {
			score := obj * float64(raw)
			if score > bestScore {
				bestScore = score
				bestClass = classIdx
			}
		}
		if bestScore <= scoreThreshold {
			continue
		}
		candidates = append(candidates, detectionCandidate{
			x1:      (cx - w/2) / ratio,
			y1:      (cy - h/2) / ratio,
			x2:      (cx + w/2) / ratio,
			y2:      (cy + h/2) / ratio,
			score:   bestScore,
			classID: bestClass,
		})
	}

	kept := nmsDetectionCandidates(candidates, nmsThreshold)
	out := make([]DetectionBox, 0, len(kept))
	for _, candidate := range kept {
		out = append(out, DetectionBox{
			X1:      clipMin(candidate.x1),
			Y1:      clipMin(candidate.y1),
			X2:      clipMax(candidate.x2, imageWidth),
			Y2:      clipMax(candidate.y2, imageHeight),
			Score:   candidate.score,
			ClassID: candidate.classID,
		})
	}
	return out, nil
}

func detectionRows(data []float32, shape ort.Shape) (int, int, error) {
	if len(shape) == 3 {
		rows := int(shape[1])
		cols := int(shape[2])
		if int(shape[0]) != 1 {
			return 0, 0, fmt.Errorf("unsupported detection batch size %d", shape[0])
		}
		if rows <= 0 || cols < 6 {
			return 0, 0, fmt.Errorf("invalid detection output shape %v", shape)
		}
		if len(data) != rows*cols {
			return 0, 0, fmt.Errorf("detection data length %d does not match shape %v", len(data), shape)
		}
		return rows, cols, nil
	}
	if len(shape) == 2 {
		rows := int(shape[0])
		cols := int(shape[1])
		if rows <= 0 || cols < 6 {
			return 0, 0, fmt.Errorf("invalid detection output shape %v", shape)
		}
		if len(data) != rows*cols {
			return 0, 0, fmt.Errorf("detection data length %d does not match shape %v", len(data), shape)
		}
		return rows, cols, nil
	}
	return 0, 0, fmt.Errorf("unsupported detection output shape %v", shape)
}

type detectionGrid struct {
	x      int
	y      int
	stride int
}

func detectionGrids(inputSize int) []detectionGrid {
	strides := []int{8, 16, 32}
	grids := make([]detectionGrid, 0)
	for _, stride := range strides {
		hsize := inputSize / stride
		wsize := inputSize / stride
		for y := 0; y < hsize; y++ {
			for x := 0; x < wsize; x++ {
				grids = append(grids, detectionGrid{x: x, y: y, stride: stride})
			}
		}
	}
	return grids
}

func nmsDetectionCandidates(candidates []detectionCandidate, threshold float64) []detectionCandidate {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	kept := make([]detectionCandidate, 0, len(candidates))
	suppressed := make([]bool, len(candidates))
	for i, candidate := range candidates {
		if suppressed[i] {
			continue
		}
		kept = append(kept, candidate)
		for j := i + 1; j < len(candidates); j++ {
			if suppressed[j] {
				continue
			}
			if detectionIOU(candidate, candidates[j]) > threshold {
				suppressed[j] = true
			}
		}
	}
	return kept
}

func detectionIOU(a detectionCandidate, b detectionCandidate) float64 {
	x1 := math.Max(a.x1, b.x1)
	y1 := math.Max(a.y1, b.y1)
	x2 := math.Min(a.x2, b.x2)
	y2 := math.Min(a.y2, b.y2)
	w := math.Max(0, x2-x1+1)
	h := math.Max(0, y2-y1+1)
	inter := w * h
	areaA := (a.x2 - a.x1 + 1) * (a.y2 - a.y1 + 1)
	areaB := (b.x2 - b.x1 + 1) * (b.y2 - b.y1 + 1)
	denom := areaA + areaB - inter
	if denom <= 0 {
		return 0
	}
	return inter / denom
}

func clipMin(value float64) int {
	if value < 0 {
		return 0
	}
	return int(value)
}

func clipMax(value float64, maxValue int) int {
	if value > float64(maxValue) {
		return maxValue
	}
	return int(value)
}
