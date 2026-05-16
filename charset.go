package goddddocr

import (
	"encoding/json"
	"fmt"
	"os"
)

func loadCharset(model Model, customPath string) ([]string, error) {
	if customPath != "" {
		data, err := os.ReadFile(customPath)
		if err != nil {
			return nil, fmt.Errorf("read custom charset %q: %w", customPath, err)
		}
		return parseCharset(data, customPath)
	}

	path := "assets/charsets/old.json"
	if model == ModelBeta {
		path = "assets/charsets/beta.json"
	}

	data, err := embeddedFiles.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read charset %q: %w", path, err)
	}

	return parseCharset(data, path)
}

func parseCharset(data []byte, source string) ([]string, error) {
	var charset []string
	if err := json.Unmarshal(data, &charset); err != nil {
		return nil, fmt.Errorf("decode charset %q: %w", source, err)
	}
	if len(charset) == 0 || charset[0] != "" {
		return nil, fmt.Errorf("charset %q is not compatible with CTC blank index 0", source)
	}
	return charset, nil
}
