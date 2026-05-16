package main

import (
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	config, err := parseFlags([]string{
		"-model", "custom",
		"-model-path", "/tmp/model.onnx",
		"-charset-path", "/tmp/charset.json",
		"-input-name", "image",
		"-output-name", "logits",
		"-image", "sample.png",
		"-expect", "abcd",
		"-json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Model != "custom" {
		t.Fatalf("model = %q", config.Model)
	}
	if config.ModelPath != "/tmp/model.onnx" || config.CharsetPath != "/tmp/charset.json" {
		t.Fatalf("unexpected custom paths: %#v", config)
	}
	if config.InputName != "image" || config.OutputName != "logits" {
		t.Fatalf("unexpected tensor names: %#v", config)
	}
	if config.ImagePath != "sample.png" || config.Expect != "abcd" || !config.JSONOutput {
		t.Fatalf("unexpected image/expect/json fields: %#v", config)
	}
}

func TestValidateConfigRejectsExpectWithoutImage(t *testing.T) {
	if err := validateConfig(doctorConfig{Expect: "abcd"}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRunDoctorReportsConfigErrors(t *testing.T) {
	report := runDoctor(doctorConfig{Model: "missing"})
	if report.OK {
		t.Fatal("expected failed report")
	}
	if !strings.Contains(report.Error, "unsupported model") {
		t.Fatalf("unexpected error: %q", report.Error)
	}
	if report.Platform == "" {
		t.Fatal("expected platform in report")
	}
}
