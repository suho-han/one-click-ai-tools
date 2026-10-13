package ui

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/spf13/viper"
)

//go:embed assets/icons/*.png
var iconFS embed.FS

var (
	rendererOnce   sync.Once
	rendererChoice iconRenderer
	iconStyleOnce  sync.Once
	iconStyle      string

	iconCacheMu sync.RWMutex
	iconCache   = map[string]image.Image{}
)

func getIconStyle() string {
	iconStyleOnce.Do(func() {
		style := strings.ToLower(viper.GetString("icon_style"))
		if style == "" {
			style = strings.ToLower(os.Getenv("OCT_ICON_STYLE"))
		}
		if style != "half-block" {
			style = "braille" // default
		}
		iconStyle = style
	})
	return iconStyle
}

type iconRenderer string

const (
	rendererNativeImage iconRenderer = "native_image"
	rendererAnsiAsset   iconRenderer = "ansi_asset"
	rendererText        iconRenderer = "text"

	// alphaTransparentThreshold is the 16-bit alpha value below which a pixel
	// is treated as fully transparent and rendered as empty space.
	alphaTransparentThreshold = uint32(0x1010)
	// alphaOpaqueThreshold is the 16-bit alpha value at or above which a pixel
	// is considered opaque enough to set a braille dot.
	alphaOpaqueThreshold = uint32(0x6000)
	// brailleBase is the Unicode code point for the empty braille pattern;
	// individual dot bits are OR-ed in to select the final character.
	brailleBase = rune(0x2800)

	// ansiFGSeq is the ANSI SGR escape sequence that sets the foreground to the
	// ansiFG/ansiBG neutral palette used for all icon fallback rendering.
	ansiFGSeq = "\x1b[38;2;214;214;232m"
	// ansiFGBGSeq sets both foreground and background to the neutral palette,
	// used when both halves of a half-block cell are opaque.
	ansiFGBGSeq = "\x1b[38;2;214;214;232;48;2;214;214;232m"

	// alpha16Max is the maximum 16-bit alpha/channel value returned by image.Color.RGBA();
	// used as the divisor when un-premultiplying alpha from sampled pixel components.
	alpha16Max = uint32(0xffff)
)

// InlineIcon returns a compact ANSI icon for inline list rendering.
func InlineIcon(name string, width int) string {
	if getRendererChoice() == rendererText {
		return ""
	}
	img, err := renderIconPNG(name)
	if err != nil {
		return ""
	}

	if getIconStyle() == "half-block" {
		return buildHalfBlockInlineIcon(img, width)
	}
	return buildBrailleInlineIcon(img, width)
}

// InlineIconLines returns a compact multi-line ANSI icon for list rendering.
func InlineIconLines(name string, width, lines int) []string {
	if getRendererChoice() == rendererText {
		return nil
	}
	img, err := renderIconPNG(name)
	if err != nil {
		return nil
	}

	if getIconStyle() == "half-block" {
		return buildHalfBlockInlineIconLines(img, width, lines)
	}
	return buildBrailleInlineIconLines(img, width, lines)
}

func renderIconPNG(name string) (image.Image, error) {
	// Icons are embedded constants; decode each once and reuse. InlineIcon
	// rendering runs per tool inside table loops, so re-reading and
	// re-decoding the PNG each call is measurable startup cost.
	iconCacheMu.RLock()
	img, ok := iconCache[name]
	iconCacheMu.RUnlock()
	if ok {
		return img, nil
	}

	p := getEmbeddedIconPath(name)
	if p == "" {
		return nil, fmt.Errorf("empty icon path")
	}
	data, err := iconFS.ReadFile(p)
	if err != nil {
		return nil, err
	}
	img, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	iconCacheMu.Lock()
	iconCache[name] = img
	iconCacheMu.Unlock()
	return img, nil
}

func getRendererChoice() iconRenderer {
	rendererOnce.Do(func() {
		rendererChoice = detectRendererChoice()
	})
	return rendererChoice
}

func detectRendererChoice() iconRenderer {
	if v := strings.TrimSpace(os.Getenv("OCT_ICON_RENDERER")); v != "" {
		switch strings.ToLower(v) {
		case string(rendererNativeImage):
			return rendererNativeImage
		case string(rendererAnsiAsset):
			return rendererAnsiAsset
		case string(rendererText):
			return rendererText
		}
	}

	if v := strings.TrimSpace(os.Getenv("OCT_IMAGE_ICONS")); v != "" {
		v = strings.ToLower(v)
		if v == "1" || v == "true" || v == "yes" || v == "on" {
			return rendererNativeImage
		}
		return rendererAnsiAsset
	}

	type terminalCapability struct {
		BestRenderer string `json:"best_renderer"`
	}

	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".oct", "terminal-capabilities.json")
		data, readErr := os.ReadFile(p)
		if readErr == nil {
			var cap terminalCapability
			if json.Unmarshal(data, &cap) == nil {
				switch cap.BestRenderer {
				case string(rendererNativeImage):
					return rendererNativeImage
				case string(rendererAnsiAsset):
					return rendererAnsiAsset
				case string(rendererText):
					return rendererText
				}
			}
		}
	}

	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	term := strings.ToLower(os.Getenv("TERM"))
	terminalEmulator := strings.ToLower(os.Getenv("TERMINAL_EMULATOR"))

	isKnownImageTerminal := termProgram == "iterm.app" ||
		termProgram == "wezterm" ||
		termProgram == "ghostty" ||
		terminalEmulator == "kitty" ||
		strings.Contains(term, "kitty")

	hasMultiplexer := os.Getenv("TMUX") != "" ||
		os.Getenv("STY") != "" ||
		os.Getenv("ZELLIJ") != "" ||
		os.Getenv("CMUX") != "" ||
		strings.Contains(term, "tmux") ||
		strings.Contains(term, "screen") ||
		strings.Contains(term, "cmux")

	if isKnownImageTerminal && !hasMultiplexer {
		return rendererNativeImage
	}

	// Conservative default: on unknown/limited terminals, hide icons
	// instead of printing low-fidelity ANSI glyphs.
	return rendererText
}

func getEmbeddedIconPath(name string) string {
	slug := iconSlug(name)
	if slug == "" {
		return ""
	}
	return fmt.Sprintf("assets/icons/%s.png", slug)
}

func buildHalfBlockInlineIcon(img image.Image, targetWidth int) string {
	b := nonTransparentBounds(img, img.Bounds())
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return ""
	}
	if targetWidth <= 0 {
		targetWidth = 4
	}
	stepX := maxInt(w/targetWidth, 1)
	stepY := stepX
	yTop := b.Min.Y + h/3
	yBot := yTop + stepY
	if yBot >= b.Max.Y {
		yBot = b.Max.Y - 1
	}

	var out strings.Builder
	for x := b.Min.X; x < b.Max.X; x += stepX {
		_, _, _, topA := sampleRGBA(img, x, yTop)
		_, _, _, botA := sampleRGBA(img, x, yBot)
		switch {
		case topA < alphaTransparentThreshold && botA < alphaTransparentThreshold:
			out.WriteByte(' ')
		case topA >= alphaTransparentThreshold && botA < alphaTransparentThreshold:
			out.WriteString(ansiFGSeq + "▀\x1b[0m")
		case topA < alphaTransparentThreshold && botA >= alphaTransparentThreshold:
			out.WriteString(ansiFGSeq + "▄\x1b[0m")
		default:
			out.WriteString(ansiFGBGSeq + "▀\x1b[0m")
		}
	}
	return out.String()
}

func buildHalfBlockInlineIconLines(img image.Image, targetWidth, lines int) []string {
	b := nonTransparentBounds(img, img.Bounds())
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || lines <= 0 {
		return nil
	}
	if targetWidth <= 0 {
		targetWidth = 4
	}
	stepX := maxInt(w/targetWidth, 1)
	stepY := maxInt(h/(lines*2), 1)

	out := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		yTop := b.Min.Y + i*stepY*2
		yBot := yTop + stepY
		if yTop >= b.Max.Y {
			yTop = b.Max.Y - 1
		}
		if yBot >= b.Max.Y {
			yBot = b.Max.Y - 1
		}
		var line strings.Builder
		for x := b.Min.X; x < b.Max.X; x += stepX {
			_, _, _, topA := sampleRGBA(img, x, yTop)
			_, _, _, botA := sampleRGBA(img, x, yBot)
			switch {
			case topA < alphaTransparentThreshold && botA < alphaTransparentThreshold:
				line.WriteByte(' ')
			case topA >= alphaTransparentThreshold && botA < alphaTransparentThreshold:
				line.WriteString(ansiFGSeq + "▀\x1b[0m")
			case topA < alphaTransparentThreshold && botA >= alphaTransparentThreshold:
				line.WriteString(ansiFGSeq + "▄\x1b[0m")
			default:
				line.WriteString(ansiFGBGSeq + "▀\x1b[0m")
			}
		}
		out = append(out, strings.TrimRight(line.String(), " "))
	}
	return out
}

func buildBrailleInlineIcon(img image.Image, targetWidth int) string {
	b := nonTransparentBounds(img, img.Bounds())
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return ""
	}
	if targetWidth <= 0 {
		targetWidth = 4
	}

	// Map full image height to 4 dots
	step := maxInt(h/4, 1)

	// Left-align horizontally
	offsetX := 0
	offsetY := 0

	var out strings.Builder
	for j := 0; j < targetWidth; j++ {
		var offset rune
		dotMap := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}
		for dy := 0; dy < 4; dy++ {
			for dx := 0; dx < 2; dx++ {
				dotX := j*2 + dx
				dotY := dy

				px := b.Min.X - offsetX + dotX*step
				py := b.Min.Y - offsetY + dotY*step

				if px >= b.Min.X && px < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
					_, _, _, a := sampleRGBA(img, px, py)
					if a >= alphaOpaqueThreshold {
						offset |= dotMap[dy][dx]
					}
				}
			}
		}
		if offset == 0 {
			out.WriteByte(' ')
		} else {
			out.WriteString(fmt.Sprintf("%s%c\x1b[0m", ansiFGSeq, brailleBase+offset))
		}
	}
	return out.String()
}

func buildBrailleInlineIconLines(img image.Image, targetWidth, lines int) []string {
	// 1. Get tight bounds
	b := nonTransparentBounds(img, img.Bounds())
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || lines <= 0 {
		return nil
	}

	// Determine the square size based on the larger dimension to preserve aspect ratio
	size := maxInt(w, h)
	// Left-align horizontally, center vertically
	offsetX := 0
	offsetY := (size - h) / 2

	if targetWidth <= 0 {
		targetWidth = 4
	}

	// 2. Use a consistent step for both axes to prevent distortion
	step := maxInt(size/(lines*4), 1)

	out := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		var line strings.Builder
		for j := 0; j < targetWidth; j++ {
			var offset rune
			dotMap := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					// Map dot to image coordinates
					dotX := j*2 + dx
					dotY := i*4 + dy

					px := b.Min.X - offsetX + dotX*step
					py := b.Min.Y - offsetY + dotY*step

					if px >= b.Min.X && px < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
						_, _, _, a := sampleRGBA(img, px, py)
						if a >= alphaOpaqueThreshold {
							offset |= dotMap[dy][dx]
						}
					}
				}
			}

			if offset == 0 {
				line.WriteByte(' ')
			} else {
				line.WriteString(fmt.Sprintf("%s%c\x1b[0m", ansiFGSeq, brailleBase+offset))
			}
		}
		out = append(out, line.String())
	}
	return out
}

func sampleRGBA(img image.Image, x, y int) (uint32, uint32, uint32, uint32) {
	r, g, b, a := img.At(x, y).RGBA()
	if a >= alphaTransparentThreshold && a < alpha16Max {
		r = r * alpha16Max / a
		g = g * alpha16Max / a
		b = b * alpha16Max / a
	}
	return r, g, b, a
}

func nonTransparentBounds(img image.Image, b image.Rectangle) image.Rectangle {
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	found := false

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a >= alphaTransparentThreshold {
				found = true
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if !found {
		return b
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func iconSlug(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
