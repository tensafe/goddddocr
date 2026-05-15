package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const defaultVersion = "1.25.0"

type target struct {
	GOOS       string
	GOARCH     string
	AssetOS    string
	AssetArch  string
	ArchiveExt string
	OutputName string
	LibMatch   string
}

func main() {
	version := flag.String("version", defaultVersion, "ONNX Runtime version")
	goos := flag.String("goos", runtime.GOOS, "target GOOS")
	goarch := flag.String("goarch", runtime.GOARCH, "target GOARCH")
	outRoot := flag.String("out", "third_party/onnxruntime", "output root directory")
	flag.Parse()

	t, err := resolveTarget(*goos, *goarch)
	if err != nil {
		log.Fatal(err)
	}

	url := fmt.Sprintf(
		"https://github.com/microsoft/onnxruntime/releases/download/v%s/onnxruntime-%s-%s-%s%s",
		*version,
		t.AssetOS,
		t.AssetArch,
		*version,
		t.ArchiveExt,
	)

	log.Printf("downloading %s", url)
	archivePath, err := download(url)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(archivePath)

	outDir := filepath.Join(*outRoot, t.GOOS+"_"+t.GOARCH)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	outPath := filepath.Join(outDir, t.OutputName)

	if strings.HasSuffix(t.ArchiveExt, ".zip") {
		err = extractZipLibrary(archivePath, outPath, t.LibMatch)
	} else {
		err = extractTarGzLibrary(archivePath, outPath, t.LibMatch)
	}
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("installed %s", outPath)
}

func resolveTarget(goos, goarch string) (target, error) {
	output := map[string]string{
		"windows": "onnxruntime.dll",
		"darwin":  "onnxruntime.dylib",
		"linux":   "libonnxruntime.so",
	}
	libMatch := map[string]string{
		"windows": ".dll",
		"darwin":  ".dylib",
		"linux":   ".so",
	}

	t := target{
		GOOS:       goos,
		GOARCH:     goarch,
		OutputName: output[goos],
		LibMatch:   libMatch[goos],
	}
	if t.OutputName == "" {
		return target{}, fmt.Errorf("unsupported GOOS %q", goos)
	}

	switch goos {
	case "darwin":
		t.AssetOS = "osx"
		t.ArchiveExt = ".tgz"
		switch goarch {
		case "amd64":
			t.AssetArch = "x86_64"
		case "arm64":
			t.AssetArch = "arm64"
		default:
			return target{}, fmt.Errorf("unsupported darwin GOARCH %q", goarch)
		}
	case "linux":
		t.AssetOS = "linux"
		t.ArchiveExt = ".tgz"
		switch goarch {
		case "amd64":
			t.AssetArch = "x64"
		case "arm64":
			t.AssetArch = "aarch64"
		default:
			return target{}, fmt.Errorf("unsupported linux GOARCH %q", goarch)
		}
	case "windows":
		t.AssetOS = "win"
		t.ArchiveExt = ".zip"
		switch goarch {
		case "amd64":
			t.AssetArch = "x64"
		case "arm64":
			t.AssetArch = "arm64"
		default:
			return target{}, fmt.Errorf("unsupported windows GOARCH %q", goarch)
		}
	}
	return t, nil
}

func download(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "onnxruntime-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func extractZipLibrary(archivePath, outPath, libMatch string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if !isRuntimeLibrary(f.Name, libMatch) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return writeFile(outPath, rc, 0o755)
	}
	return fmt.Errorf("runtime library matching %q not found in zip", libMatch)
}

func extractTarGzLibrary(archivePath, outPath, libMatch string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if !isRuntimeLibrary(header.Name, libMatch) {
			continue
		}
		return writeFile(outPath, tr, 0o755)
	}
	return fmt.Errorf("runtime library matching %q not found in tgz", libMatch)
}

func isRuntimeLibrary(name, libMatch string) bool {
	base := filepath.Base(name)
	if !strings.Contains(base, "onnxruntime") {
		return false
	}
	switch libMatch {
	case ".dll":
		return strings.EqualFold(base, "onnxruntime.dll")
	case ".dylib":
		return strings.HasSuffix(base, ".dylib")
	case ".so":
		return strings.Contains(base, ".so")
	default:
		return strings.Contains(base, libMatch)
	}
}

func writeFile(path string, src io.Reader, mode os.FileMode) error {
	tmp := path + ".tmp"
	dst, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
