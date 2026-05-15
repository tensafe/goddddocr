package goddddocr

import (
	"encoding/base64"
	"testing"
)

func TestDecodeBase64ImageDataURL(t *testing.T) {
	want := []byte("image")
	got, err := decodeBase64Image("data:image/png;base64," + base64.StdEncoding.EncodeToString(want))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("decoded mismatch: got %q, want %q", got, want)
	}
}

func TestParseBool(t *testing.T) {
	truthy, err := parseBool("yes")
	if err != nil || !truthy {
		t.Fatalf("yes should parse true, got %v %v", truthy, err)
	}

	falsey, err := parseBool("0")
	if err != nil || falsey {
		t.Fatalf("0 should parse false, got %v %v", falsey, err)
	}

	if _, err := parseBool("maybe"); err == nil {
		t.Fatal("expected parse error")
	}
}
