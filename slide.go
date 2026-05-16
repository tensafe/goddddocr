package goddddocr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const slideComparisonThreshold = 30

type SlideResult struct {
	Target  []int `json:"target"`
	TargetX int   `json:"target_x"`
	TargetY int   `json:"target_y"`
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
