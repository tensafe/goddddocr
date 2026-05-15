//go:build !(darwin && arm64)

package goddddocr

func bundledSharedLibrary() (name string, data []byte, ok bool, err error) {
	return "", nil, false, nil
}
