package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseOCRCommandFlags(t *testing.T) {
	config, err := parseOCRCommandFlags([]string{
		"--file", "captcha.png",
		"--json",
		"--confidence",
		"--probability",
		"--charset-range", "0123456789",
		"--model", "custom",
		"--model-path", "/tmp/model.onnx",
		"--charset-path", "/tmp/charset.json",
		"--input-name", "image",
		"--output-name", "logits",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if config.FilePath != "captcha.png" {
		t.Fatalf("file = %q", config.FilePath)
	}
	if !config.JSONOutput || !config.Confidence || !config.Probability {
		t.Fatalf("unexpected output flags: %#v", config)
	}
	if config.CharsetRange != "0123456789" {
		t.Fatalf("charset range = %q", config.CharsetRange)
	}
	if config.Model != "custom" || config.ModelPath != "/tmp/model.onnx" || config.CharsetPath != "/tmp/charset.json" {
		t.Fatalf("unexpected model config: %#v", config)
	}
	if config.InputName != "image" || config.OutputName != "logits" {
		t.Fatalf("unexpected tensor names: %#v", config)
	}
}

func TestParseOCRCommandFlagsAcceptsImageAlias(t *testing.T) {
	config, err := parseOCRCommandFlags([]string{"--image", "captcha.png"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if config.FilePath != "captcha.png" {
		t.Fatalf("file = %q", config.FilePath)
	}
}

func TestParseOCRCommandFlagsRejectsMissingFile(t *testing.T) {
	if _, err := parseOCRCommandFlags([]string{"--json"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestRunRootCommandRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRootCommand([]string{"missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunOCRCommandReportsMissingFileAsJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRootCommand([]string{"ocr", "--file", "definitely-missing.png", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ok": false`) || !strings.Contains(stdout.String(), `"error": "read image:`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
