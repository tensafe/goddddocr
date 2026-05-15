package goddddocr

import (
	"encoding/json"
	"fmt"
)

func loadCharset(model Model) ([]string, error) {
	path := "assets/charsets/old.json"
	if model == ModelBeta {
		path = "assets/charsets/beta.json"
	}

	data, err := embeddedFiles.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read charset %q: %w", path, err)
	}

	var charset []string
	if err := json.Unmarshal(data, &charset); err != nil {
		return nil, fmt.Errorf("decode charset %q: %w", path, err)
	}
	if len(charset) == 0 || charset[0] != "" {
		return nil, fmt.Errorf("charset %q is not compatible with CTC blank index 0", path)
	}
	return charset, nil
}
