package goddddocr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
)

const slideComparisonThreshold = 30
const slideEdgeThreshold = 80

type SlideResult struct {
	Target     []int   `json:"target"`
	TargetX    int     `json:"target_x"`
	TargetY    int     `json:"target_y"`
	Confidence float64 `json:"confidence,omitempty"`
}

func SlideComparisonBytes(targetData []byte, backgroundData []byte) (*SlideResult, error) {
	target, _, err := image.Decode(bytes.NewReader(targetData))
	if err != nil {
		return nil, fmt.Errorf("decode target image: %w", err)
	}
	background, _, err := image.Decode(bytes.NewReader(backgroundData))
	if err != nil {
		return nil, fmt.Errorf("decode background image: %w", err)
	}
	return SlideComparisonImages(target, background)
}

func SlideMatchBytes(targetData []byte, backgroundData []byte, simpleTarget bool) (*SlideResult, error) {
	target, _, err := image.Decode(bytes.NewReader(targetData))
	if err != nil {
		return nil, fmt.Errorf("decode target image: %w", err)
	}
	background, _, err := image.Decode(bytes.NewReader(backgroundData))
	if err != nil {
		return nil, fmt.Errorf("decode background image: %w", err)
	}
	return SlideMatchImages(target, background, simpleTarget)
}

func SlideComparisonImages(target image.Image, background image.Image) (*SlideResult, error) {
	if target == nil || background == nil {
		return nil, fmt.Errorf("target and background images are required")
	}
	targetBounds := target.Bounds()
	backgroundBounds := background.Bounds()
	width := targetBounds.Dx()
	height := targetBounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("empty target image")
	}
	if backgroundBounds.Dx() != width || backgroundBounds.Dy() != height {
		return nil, fmt.Errorf("target dimensions %dx%d do not match background %dx%d", width, height, backgroundBounds.Dx(), backgroundBounds.Dy())
	}

	binary := slideDiffBinary(target, background, targetBounds, backgroundBounds, width, height)
	filtered := morphologyOpen(morphologyClose(binary, width, height), width, height)
	component, ok := largestBinaryComponent(filtered, width, height)
	if !ok {
		return &SlideResult{Target: []int{0, 0}}, nil
	}
	centerX := component.minX + (component.maxX-component.minX+1)/2
	centerY := component.minY + (component.maxY-component.minY+1)/2
	return &SlideResult{
		Target:  []int{centerX, centerY},
		TargetX: centerX,
		TargetY: centerY,
	}, nil
}

func SlideMatchImages(target image.Image, background image.Image, simpleTarget bool) (*SlideResult, error) {
	if target == nil || background == nil {
		return nil, fmt.Errorf("target and background images are required")
	}
	targetBounds := target.Bounds()
	backgroundBounds := background.Bounds()
	targetWidth := targetBounds.Dx()
	targetHeight := targetBounds.Dy()
	backgroundWidth := backgroundBounds.Dx()
	backgroundHeight := backgroundBounds.Dy()
	if targetWidth <= 0 || targetHeight <= 0 {
		return nil, fmt.Errorf("empty target image")
	}
	if backgroundWidth <= 0 || backgroundHeight <= 0 {
		return nil, fmt.Errorf("empty background image")
	}
	if targetWidth > backgroundWidth || targetHeight > backgroundHeight {
		return nil, fmt.Errorf("target dimensions %dx%d exceed background %dx%d", targetWidth, targetHeight, backgroundWidth, backgroundHeight)
	}

	targetGray := imageToGrayBuffer(target, targetBounds)
	backgroundGray := imageToGrayBuffer(background, backgroundBounds)
	matchTarget := targetGray
	matchBackground := backgroundGray
	if !simpleTarget {
		targetEdges := sobelEdges(targetGray, targetWidth, targetHeight)
		backgroundEdges := sobelEdges(backgroundGray, backgroundWidth, backgroundHeight)
		if hasNonZeroByte(targetEdges) && hasNonZeroByte(backgroundEdges) {
			matchTarget = targetEdges
			matchBackground = backgroundEdges
		}
	}

	x, y, confidence := matchTemplateCCOEFFNormed(matchBackground, backgroundWidth, backgroundHeight, matchTarget, targetWidth, targetHeight)
	centerX := x + targetWidth/2
	centerY := y + targetHeight/2
	return &SlideResult{
		Target:     []int{centerX, centerY},
		TargetX:    centerX,
		TargetY:    centerY,
		Confidence: confidence,
	}, nil
}

func slideDiffBinary(target image.Image, background image.Image, targetBounds image.Rectangle, backgroundBounds image.Rectangle, width int, height int) []bool {
	binary := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			targetColor := color.NRGBAModel.Convert(target.At(targetBounds.Min.X+x, targetBounds.Min.Y+y)).(color.NRGBA)
			backgroundColor := color.NRGBAModel.Convert(background.At(backgroundBounds.Min.X+x, backgroundBounds.Min.Y+y)).(color.NRGBA)
			diff := grayscalePILLike(
				absUint8Diff(targetColor.R, backgroundColor.R),
				absUint8Diff(targetColor.G, backgroundColor.G),
				absUint8Diff(targetColor.B, backgroundColor.B),
			)
			binary[y*width+x] = diff > slideComparisonThreshold
		}
	}
	return binary
}

func imageToGrayBuffer(img image.Image, bounds image.Rectangle) []uint8 {
	width := bounds.Dx()
	height := bounds.Dy()
	gray := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			gray[y*width+x] = grayscalePILLike(c.R, c.G, c.B)
		}
	}
	return gray
}

func sobelEdges(gray []uint8, width int, height int) []uint8 {
	edges := make([]uint8, len(gray))
	if width < 3 || height < 3 {
		return edges
	}
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			idx := y*width + x
			gx := -int(gray[idx-width-1]) + int(gray[idx-width+1]) -
				2*int(gray[idx-1]) + 2*int(gray[idx+1]) -
				int(gray[idx+width-1]) + int(gray[idx+width+1])
			gy := -int(gray[idx-width-1]) - 2*int(gray[idx-width]) - int(gray[idx-width+1]) +
				int(gray[idx+width-1]) + 2*int(gray[idx+width]) + int(gray[idx+width+1])
			magnitude := math.Sqrt(float64(gx*gx + gy*gy))
			if magnitude >= slideEdgeThreshold {
				edges[idx] = 255
			}
		}
	}
	return edges
}

func hasNonZeroByte(data []uint8) bool {
	for _, value := range data {
		if value != 0 {
			return true
		}
	}
	return false
}

func matchTemplateCCOEFFNormed(background []uint8, backgroundWidth int, backgroundHeight int, target []uint8, targetWidth int, targetHeight int) (int, int, float64) {
	targetMean, targetEnergy := meanAndCenteredEnergy(target)
	bestX := 0
	bestY := 0
	bestScore := math.Inf(-1)

	for y := 0; y <= backgroundHeight-targetHeight; y++ {
		for x := 0; x <= backgroundWidth-targetWidth; x++ {
			patchMean := patchMean(background, backgroundWidth, x, y, targetWidth, targetHeight)
			var numerator float64
			var patchEnergy float64
			for ty := 0; ty < targetHeight; ty++ {
				backgroundOffset := (y+ty)*backgroundWidth + x
				targetOffset := ty * targetWidth
				for tx := 0; tx < targetWidth; tx++ {
					targetDelta := float64(target[targetOffset+tx]) - targetMean
					patchDelta := float64(background[backgroundOffset+tx]) - patchMean
					numerator += targetDelta * patchDelta
					patchEnergy += patchDelta * patchDelta
				}
			}
			score := normalizedScore(numerator, targetEnergy, patchEnergy)
			if score > bestScore {
				bestX = x
				bestY = y
				bestScore = score
			}
		}
	}
	if math.IsInf(bestScore, -1) {
		return 0, 0, 0
	}
	return bestX, bestY, bestScore
}

func meanAndCenteredEnergy(values []uint8) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, value := range values {
		sum += float64(value)
	}
	mean := sum / float64(len(values))
	var energy float64
	for _, value := range values {
		delta := float64(value) - mean
		energy += delta * delta
	}
	return mean, energy
}

func patchMean(background []uint8, backgroundWidth int, x int, y int, width int, height int) float64 {
	var sum float64
	for yy := 0; yy < height; yy++ {
		offset := (y+yy)*backgroundWidth + x
		for xx := 0; xx < width; xx++ {
			sum += float64(background[offset+xx])
		}
	}
	return sum / float64(width*height)
}

func normalizedScore(numerator float64, targetEnergy float64, patchEnergy float64) float64 {
	if targetEnergy == 0 || patchEnergy == 0 {
		return 0
	}
	return numerator / math.Sqrt(targetEnergy*patchEnergy)
}

func absUint8Diff(a uint8, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func morphologyClose(src []bool, width int, height int) []bool {
	return erode3x3(dilate3x3(src, width, height), width, height)
}

func morphologyOpen(src []bool, width int, height int) []bool {
	return dilate3x3(erode3x3(src, width, height), width, height)
}

func dilate3x3(src []bool, width int, height int) []bool {
	out := make([]bool, len(src))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			found := false
			for yy := maxInt(0, y-1); yy <= minInt(height-1, y+1) && !found; yy++ {
				for xx := maxInt(0, x-1); xx <= minInt(width-1, x+1); xx++ {
					if src[yy*width+xx] {
						found = true
						break
					}
				}
			}
			out[y*width+x] = found
		}
	}
	return out
}

func erode3x3(src []bool, width int, height int) []bool {
	out := make([]bool, len(src))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			all := true
			for yy := maxInt(0, y-1); yy <= minInt(height-1, y+1) && all; yy++ {
				for xx := maxInt(0, x-1); xx <= minInt(width-1, x+1); xx++ {
					if !src[yy*width+xx] {
						all = false
						break
					}
				}
			}
			out[y*width+x] = all
		}
	}
	return out
}

type binaryComponent struct {
	minX int
	minY int
	maxX int
	maxY int
	area int
}

func largestBinaryComponent(binary []bool, width int, height int) (binaryComponent, bool) {
	visited := make([]bool, len(binary))
	best := binaryComponent{}
	found := false
	queue := make([]int, 0)
	for idx, on := range binary {
		if !on || visited[idx] {
			continue
		}
		component := floodBinaryComponent(binary, visited, width, height, idx, queue)
		if !found || component.area > best.area {
			best = component
			found = true
		}
	}
	return best, found
}

func floodBinaryComponent(binary []bool, visited []bool, width int, height int, start int, scratch []int) binaryComponent {
	queue := append(scratch[:0], start)
	visited[start] = true
	startX := start % width
	startY := start / width
	component := binaryComponent{minX: startX, minY: startY, maxX: startX, maxY: startY}
	for head := 0; head < len(queue); head++ {
		idx := queue[head]
		x := idx % width
		y := idx / width
		component.area++
		if x < component.minX {
			component.minX = x
		}
		if y < component.minY {
			component.minY = y
		}
		if x > component.maxX {
			component.maxX = x
		}
		if y > component.maxY {
			component.maxY = y
		}

		for yy := maxInt(0, y-1); yy <= minInt(height-1, y+1); yy++ {
			for xx := maxInt(0, x-1); xx <= minInt(width-1, x+1); xx++ {
				next := yy*width + xx
				if visited[next] || !binary[next] {
					continue
				}
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return component
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
