//go:build darwin && arm64

package goddddocr

import "embed"

const embeddedDarwinArm64 = "third_party/onnxruntime/darwin_arm64/onnxruntime.dylib"

//go:embed third_party/onnxruntime/darwin_arm64/onnxruntime.dylib
var runtimeBundle embed.FS

func bundledSharedLibrary() (name string, data []byte, ok bool, err error) {
	data, err = runtimeBundle.ReadFile(embeddedDarwinArm64)
	if err != nil {
		return "", nil, false, err
	}
	return "onnxruntime-darwin-arm64", data, true, nil
}
