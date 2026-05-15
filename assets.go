package goddddocr

import "embed"

//go:embed assets/models/common_old.onnx assets/models/common.onnx assets/charsets/old.json assets/charsets/beta.json
var embeddedFiles embed.FS
