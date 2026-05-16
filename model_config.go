package goddddocr

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultOCRInputName  = "input1"
	defaultOCROutputName = "387"
)

type resolvedOCRConfig struct {
	model       Model
	modelPath   string
	charsetPath string
	inputName   string
	outputName  string
}

func resolveOCRConfig(config Config) (resolvedOCRConfig, error) {
	modelPath := strings.TrimSpace(config.ModelPath)
	charsetPath := strings.TrimSpace(config.CharsetPath)

	model := config.Model
	if modelPath != "" {
		model = ModelCustom
	} else if model == "" {
		model = ModelOld
	}

	switch model {
	case ModelOld, ModelBeta:
	case ModelCustom:
		if modelPath == "" {
			return resolvedOCRConfig{}, fmt.Errorf("custom model requires ModelPath")
		}
		if charsetPath == "" {
			return resolvedOCRConfig{}, fmt.Errorf("custom model requires CharsetPath")
		}
	default:
		return resolvedOCRConfig{}, fmt.Errorf("unsupported model %q", model)
	}

	inputName := strings.TrimSpace(config.InputName)
	if inputName == "" {
		inputName = defaultOCRInputName
	}
	outputName := strings.TrimSpace(config.OutputName)
	if outputName == "" {
		outputName = defaultOCROutputName
	}

	return resolvedOCRConfig{
		model:       model,
		modelPath:   modelPath,
		charsetPath: charsetPath,
		inputName:   inputName,
		outputName:  outputName,
	}, nil
}

func loadModelData(model Model, customPath string) ([]byte, string, error) {
	if customPath != "" {
		data, err := os.ReadFile(customPath)
		if err != nil {
			return nil, "", fmt.Errorf("read custom model %q: %w", customPath, err)
		}
		if len(data) == 0 {
			return nil, "", fmt.Errorf("custom model %q is empty", customPath)
		}
		return data, customPath, nil
	}

	path := "assets/models/common_old.onnx"
	if model == ModelBeta {
		path = "assets/models/common.onnx"
	}
	data, err := embeddedFiles.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read model %q: %w", path, err)
	}
	return data, path, nil
}
