package goddddocr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOCRConfigDefaults(t *testing.T) {
	config, err := resolveOCRConfig(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if config.model != ModelOld {
		t.Fatalf("model = %q, want %q", config.model, ModelOld)
	}
	if config.inputName != defaultOCRInputName {
		t.Fatalf("input name = %q, want %q", config.inputName, defaultOCRInputName)
	}
	if config.outputName != defaultOCROutputName {
		t.Fatalf("output name = %q, want %q", config.outputName, defaultOCROutputName)
	}
}

func TestResolveOCRConfigCustomModel(t *testing.T) {
	config, err := resolveOCRConfig(Config{
		ModelPath:   " /models/custom.onnx ",
		CharsetPath: " /models/charset.json ",
		InputName:   " image ",
		OutputName:  " logits ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.model != ModelCustom {
		t.Fatalf("model = %q, want %q", config.model, ModelCustom)
	}
	if config.modelPath != "/models/custom.onnx" {
		t.Fatalf("model path = %q", config.modelPath)
	}
	if config.charsetPath != "/models/charset.json" {
		t.Fatalf("charset path = %q", config.charsetPath)
	}
	if config.inputName != "image" || config.outputName != "logits" {
		t.Fatalf("session names = %q/%q", config.inputName, config.outputName)
	}
}

func TestResolveOCRConfigRejectsIncompleteCustomModel(t *testing.T) {
	if _, err := resolveOCRConfig(Config{Model: ModelCustom}); err == nil {
		t.Fatal("expected missing custom model path error")
	}
	if _, err := resolveOCRConfig(Config{ModelPath: "model.onnx"}); err == nil {
		t.Fatal("expected missing custom charset path error")
	}
	if _, err := resolveOCRConfig(Config{Model: Model("missing")}); err == nil {
		t.Fatal("expected unsupported model error")
	}
}

func TestLoadCustomCharset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "charset.json")
	if err := os.WriteFile(path, []byte(`["","a","b"]`), 0o600); err != nil {
		t.Fatal(err)
	}

	charset, err := loadCharset(ModelCustom, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(charset) != 3 || charset[0] != "" || charset[1] != "a" || charset[2] != "b" {
		t.Fatalf("unexpected charset: %#v", charset)
	}
}

func TestLoadCustomCharsetRequiresBlankIndexZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "charset.json")
	if err := os.WriteFile(path, []byte(`["a","b"]`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadCharset(ModelCustom, path); err == nil {
		t.Fatal("expected blank-index validation error")
	}
}
