package goddddocr

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
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

type ClassifyOptions struct {
	PNGFix *bool
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
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}
	return o.ClassifyImage(img, options)
}

func (o *OCR) ClassifyImage(img image.Image, options *ClassifyOptions) (string, error) {
	if o == nil || o.session == nil {
		return "", fmt.Errorf("OCR engine is closed")
	}

	pngFix := o.pngFix
	if options != nil && options.PNGFix != nil {
		pngFix = *options.PNGFix
	}

	inputData, width, err := preprocessOCRImage(img, pngFix)
	if err != nil {
		return "", err
	}

	inputTensor, err := ort.NewTensor(ort.NewShape(1, 1, ocrTargetHeight, int64(width)), inputData)
	if err != nil {
		return "", fmt.Errorf("create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputs := []ort.Value{nil}
	o.mu.Lock()
	err = o.session.Run([]ort.Value{inputTensor}, outputs)
	o.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("run OCR model: %w", err)
	}
	if outputs[0] == nil {
		return "", fmt.Errorf("OCR model returned no output")
	}
	defer outputs[0].Destroy()

	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return "", fmt.Errorf("unexpected OCR output tensor type %T", outputs[0])
	}

	return decodeOCRText(outputTensor.GetData(), outputTensor.GetShape(), o.chars)
}

func decodeOCRText(data []float32, shape ort.Shape, charset []string) (string, error) {
	if len(shape) == 0 {
		return "", fmt.Errorf("empty OCR output shape")
	}

	var indices []int
	switch len(shape) {
	case 3:
		d0, d1, d2 := int(shape[0]), int(shape[1]), int(shape[2])
		if d0 <= 0 || d1 <= 0 || d2 <= 0 {
			return "", fmt.Errorf("invalid OCR output shape %v", shape)
		}
		if d1 == 1 {
			indices = make([]int, d0)
			for t := 0; t < d0; t++ {
				indices[t] = argmax(data[t*d1*d2 : t*d1*d2+d2])
			}
		} else if d0 == 1 {
			indices = make([]int, d1)
			for t := 0; t < d1; t++ {
				offset := t * d2
				indices[t] = argmax(data[offset : offset+d2])
			}
		} else {
			indices = make([]int, d1)
			for t := 0; t < d1; t++ {
				offset := t * d2
				indices[t] = argmax(data[offset : offset+d2])
			}
		}
	case 2:
		rows, cols := int(shape[0]), int(shape[1])
		if rows <= 0 || cols <= 0 {
			return "", fmt.Errorf("invalid OCR output shape %v", shape)
		}
		indices = make([]int, rows)
		for row := 0; row < rows; row++ {
			offset := row * cols
			indices[row] = argmax(data[offset : offset+cols])
		}
	default:
		idx := argmax(data)
		indices = []int{idx}
	}

	var text strings.Builder
	prev := -1
	for _, idx := range indices {
		if idx != prev && idx != 0 && idx >= 0 && idx < len(charset) {
			text.WriteString(charset[idx])
		}
		prev = idx
	}
	return text.String(), nil
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
