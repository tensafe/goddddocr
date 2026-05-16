package goddddocr

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
)

type HSVRange struct {
	Lower [3]int `json:"lower"`
	Upper [3]int `json:"upper"`
}

type ColorFilterOptions struct {
	Colors []string   `json:"colors,omitempty"`
	Ranges []HSVRange `json:"ranges,omitempty"`
}

func NewColorFilterColors(colors ...string) *ColorFilterOptions {
	return &ColorFilterOptions{Colors: colors}
}

func NewColorFilterRanges(ranges ...HSVRange) *ColorFilterOptions {
	return &ColorFilterOptions{Ranges: ranges}
}

func (r *HSVRange) UnmarshalJSON(data []byte) error {
	var object struct {
		Lower *[3]int `json:"lower"`
		Upper *[3]int `json:"upper"`
	}
	if err := json.Unmarshal(data, &object); err == nil && (object.Lower != nil || object.Upper != nil) {
		if object.Lower == nil || object.Upper == nil {
			return fmt.Errorf("HSV range object requires lower and upper")
		}
		r.Lower = *object.Lower
		r.Upper = *object.Upper
		return validateHSVRange(*r)
	}

	var pair [2][3]int
	if err := json.Unmarshal(data, &pair); err != nil {
		return fmt.Errorf("HSV range must be [[h,s,v],[h,s,v]] or {lower,upper}")
	}
	r.Lower = pair[0]
	r.Upper = pair[1]
	return validateHSVRange(*r)
}

func (o *ColorFilterOptions) hsvRanges() ([]HSVRange, error) {
	if o == nil {
		return nil, nil
	}
	ranges := make([]HSVRange, 0, len(o.Ranges)+len(o.Colors))
	for _, colorName := range o.Colors {
		preset, ok := colorFilterPresets[strings.ToLower(strings.TrimSpace(colorName))]
		if !ok {
			return nil, fmt.Errorf("unsupported color preset %q", colorName)
		}
		ranges = append(ranges, preset...)
	}
	for _, hsvRange := range o.Ranges {
		if err := validateHSVRange(hsvRange); err != nil {
			return nil, err
		}
		ranges = append(ranges, hsvRange)
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("color filter requires at least one color or custom HSV range")
	}
	return ranges, nil
}

func applyColorFilter(img image.Image, options *ColorFilterOptions) (*image.NRGBA, error) {
	ranges, err := options.hsvRanges()
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			hsv := rgbToOpenCVHSV(c.R, c.G, c.B)
			if hsvInRanges(hsv, ranges) {
				c.A = 255
				dst.SetNRGBA(x, y, c)
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	return dst, nil
}

func validateHSVRange(hsvRange HSVRange) error {
	for idx := 0; idx < 3; idx++ {
		lower := hsvRange.Lower[idx]
		upper := hsvRange.Upper[idx]
		max := 255
		if idx == 0 {
			max = 180
		}
		if lower < 0 || lower > max || upper < 0 || upper > max {
			return fmt.Errorf("HSV range values must be within H=0..180 and S/V=0..255")
		}
		if lower > upper {
			return fmt.Errorf("HSV range lower bound cannot exceed upper bound")
		}
	}
	return nil
}

func rgbToOpenCVHSV(r, g, b uint8) [3]int {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	maxValue := math.Max(rf, math.Max(gf, bf))
	minValue := math.Min(rf, math.Min(gf, bf))
	delta := maxValue - minValue

	var hue float64
	switch {
	case delta == 0:
		hue = 0
	case maxValue == rf:
		hue = 60 * math.Mod((gf-bf)/delta, 6)
	case maxValue == gf:
		hue = 60 * ((bf-rf)/delta + 2)
	default:
		hue = 60 * ((rf-gf)/delta + 4)
	}
	if hue < 0 {
		hue += 360
	}

	saturation := 0.0
	if maxValue != 0 {
		saturation = delta / maxValue
	}

	return [3]int{
		int(math.Round(hue / 2)),
		int(math.Round(saturation * 255)),
		int(math.Round(maxValue * 255)),
	}
}

func hsvInRanges(hsv [3]int, ranges []HSVRange) bool {
	for _, hsvRange := range ranges {
		if hsv[0] >= hsvRange.Lower[0] && hsv[0] <= hsvRange.Upper[0] &&
			hsv[1] >= hsvRange.Lower[1] && hsv[1] <= hsvRange.Upper[1] &&
			hsv[2] >= hsvRange.Lower[2] && hsv[2] <= hsvRange.Upper[2] {
			return true
		}
	}
	return false
}

var colorFilterPresets = map[string][]HSVRange{
	"red": {
		{Lower: [3]int{0, 50, 50}, Upper: [3]int{10, 255, 255}},
		{Lower: [3]int{170, 50, 50}, Upper: [3]int{180, 255, 255}},
	},
	"blue":   {{Lower: [3]int{100, 50, 50}, Upper: [3]int{130, 255, 255}}},
	"green":  {{Lower: [3]int{40, 50, 50}, Upper: [3]int{80, 255, 255}}},
	"yellow": {{Lower: [3]int{20, 50, 50}, Upper: [3]int{40, 255, 255}}},
	"orange": {{Lower: [3]int{10, 50, 50}, Upper: [3]int{20, 255, 255}}},
	"purple": {{Lower: [3]int{130, 50, 50}, Upper: [3]int{170, 255, 255}}},
	"cyan":   {{Lower: [3]int{80, 50, 50}, Upper: [3]int{100, 255, 255}}},
	"black":  {{Lower: [3]int{0, 0, 0}, Upper: [3]int{180, 255, 50}}},
	"white":  {{Lower: [3]int{0, 0, 200}, Upper: [3]int{180, 30, 255}}},
	"gray":   {{Lower: [3]int{0, 0, 50}, Upper: [3]int{180, 30, 200}}},
}
