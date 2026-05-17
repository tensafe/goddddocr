package goddddocr

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	envSharedLibraryPath  = "ONNXRUNTIME_SHARED_LIBRARY_PATH"
	envRuntimeHome        = "ONNXRUNTIME_HOME"
	defaultORTVersion     = "1.25.0"
	darwinAMD64ORTVersion = "1.23.2"
)

var (
	runtimeMu          sync.Mutex
	runtimeInitialized bool
	runtimePath        string
)

// InitRuntime initializes ONNX Runtime once for the process.
//
// If sharedLibraryPath is empty, the function first checks
// ONNXRUNTIME_SHARED_LIBRARY_PATH, then ONNXRUNTIME_HOME, then a local
// third_party/onnxruntime/<GOOS>_<GOARCH>/ directory, then an embedded runtime
// if this build includes one. Finally, it asks the system dynamic loader for
// the platform's default ONNX Runtime library name.
func InitRuntime(sharedLibraryPath string) error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()

	if runtimeInitialized {
		return nil
	}

	path, source, err := resolveSharedLibrary(sharedLibraryPath)
	if err != nil {
		return err
	}
	ort.SetSharedLibraryPath(path)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("initialize ONNX Runtime from %s (%s): %w\n%s", path, source, err, runtimeInstallHint())
	}
	runtimeInitialized = true
	runtimePath = path
	return nil
}

// ShutdownRuntime releases the ONNX Runtime process-wide environment.
// Most long-running services do not need to call this until process shutdown.
func ShutdownRuntime() error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if !runtimeInitialized {
		return nil
	}
	err := ort.DestroyEnvironment()
	if err == nil {
		runtimeInitialized = false
		runtimePath = ""
	}
	return err
}

// RuntimeLibraryPath returns the path that initialized ONNX Runtime, or an
// empty string if InitRuntime has not succeeded yet.
func RuntimeLibraryPath() string {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	return runtimePath
}

func resolveSharedLibrary(explicit string) (string, string, error) {
	if explicit != "" {
		return explicit, "config", nil
	}

	if path := os.Getenv(envSharedLibraryPath); path != "" {
		return path, envSharedLibraryPath, nil
	}

	if path := findInRuntimeHome(); path != "" {
		return path, envRuntimeHome, nil
	}

	if path := findLocalSharedLibrary(); path != "" {
		return path, "third_party", nil
	}

	if path, ok, err := ensureBundledSharedLibrary(); err != nil {
		return "", "", err
	} else if ok {
		return path, "embedded", nil
	}

	return defaultSharedLibraryName(), "system loader", nil
}

func findInRuntimeHome() string {
	home := os.Getenv(envRuntimeHome)
	if home == "" {
		return ""
	}
	return firstExisting(candidatePaths(home)...)
}

func findLocalSharedLibrary() string {
	roots := []string{}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	for _, root := range roots {
		platformDir := filepath.Join(root, "third_party", "onnxruntime", platformKey())
		if path := firstExisting(candidatePaths(platformDir)...); path != "" {
			return path
		}
	}
	return ""
}

func candidatePaths(dir string) []string {
	names := candidateLibraryNames()
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func candidateLibraryNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"onnxruntime.dll"}
	case "darwin":
		return []string{
			"onnxruntime.dylib",
			"libonnxruntime.dylib",
			"libonnxruntime." + runtimeORTVersion() + ".dylib",
		}
	default:
		return []string{
			"libonnxruntime.so",
			"libonnxruntime.so." + runtimeORTVersion(),
			"onnxruntime.so",
		}
	}
}

func runtimeORTVersion() string {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		return darwinAMD64ORTVersion
	}
	return defaultORTVersion
}

func defaultSharedLibraryName() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime.dylib"
	default:
		return "libonnxruntime.so"
	}
}

func platformKey() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

func ensureBundledSharedLibrary() (string, bool, error) {
	name, data, ok, err := bundledSharedLibrary()
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}

	sum := sha256.Sum256(data)
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		cacheRoot = os.TempDir()
	}
	dir := filepath.Join(cacheRoot, "goddddocr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, fmt.Errorf("create ONNX Runtime cache directory: %w", err)
	}

	path := filepath.Join(dir, name+"-"+hex.EncodeToString(sum[:8])+sharedLibraryExt())
	if existing, err := os.ReadFile(path); err == nil && len(existing) == len(data) {
		return path, true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("inspect cached ONNX Runtime library: %w", err)
	}

	if err := os.WriteFile(path, data, 0o755); err != nil {
		return "", false, fmt.Errorf("write cached ONNX Runtime library: %w", err)
	}
	return path, true, nil
}

func sharedLibraryExt() string {
	switch runtime.GOOS {
	case "windows":
		return ".dll"
	case "darwin":
		return ".dylib"
	default:
		return ".so"
	}
}

func runtimeInstallHint() string {
	return fmt.Sprintf(`Install ONNX Runtime %s for %s/%s, then use one of:
  - pass Config.SharedLibraryPath / -onnxruntime-lib
  - export %s=/absolute/path/to/%s
  - export %s=/directory/containing/%s
  - run: go run ./cmd/ortfetch
`, runtimeORTVersion(), runtime.GOOS, runtime.GOARCH, envSharedLibraryPath, defaultSharedLibraryName(), envRuntimeHome, defaultSharedLibraryName())
}
