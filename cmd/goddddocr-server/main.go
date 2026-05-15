package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/tensafe/goddddocr"
)

func main() {
	addr := flag.String("addr", ":8088", "HTTP listen address")
	model := flag.String("model", string(goddddocr.ModelOld), "OCR model: old or beta")
	ortLib := flag.String("onnxruntime-lib", "", "path to ONNX Runtime shared library")
	pngFix := flag.Bool("png-fix", false, "composite transparent PNGs over a white background")
	flag.Parse()

	ocr, err := goddddocr.NewOCR(goddddocr.Config{
		Model:             goddddocr.Model(*model),
		SharedLibraryPath: *ortLib,
		PNGFix:            *pngFix,
	})
	if err != nil {
		log.Fatalf("init OCR: %v", err)
	}
	defer ocr.Close()

	log.Printf("goddddocr server listening on %s, model=%s", *addr, ocr.Model())
	if err := http.ListenAndServe(*addr, goddddocr.NewServer(ocr).Handler()); err != nil {
		log.Fatal(err)
	}
}
