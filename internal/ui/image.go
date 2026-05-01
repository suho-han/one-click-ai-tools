package ui

import (
	"bytes"
	"embed"
	"encoding/base64"
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
	ansiFG                           = uint32(0xD6D6)
	ansiBG                           = uint32(0xE8E8)
)

// PrintIcon renders an icon with fallback chain: native image -> ansi asset -> text.
func PrintIcon(name string, size int) {
	switch getRendererChoice() {
	case rendererNativeImage:
		if printNativeImage(name, size) {
			return
		}
		fallthrough
	case rendererAnsiAsset:
		if printANSIFromPNG(name, size) {
			return
		}
	}
	printTextFallback(name)
}

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

func printNativeImage(name string, size int) bool {
	term := os.Getenv("TERM_PROGRAM")
	isIterm := term == "iTerm.app"
	isWezterm := term == "WezTerm"
	isGhostty := term == "ghostty" || strings.Contains(strings.ToLower(os.Getenv("TERM")), "ghostty")
	isKitty := os.Getenv("TERMINAL_EMULATOR") == "kitty" || strings.Contains(strings.ToLower(os.Getenv("TERM")), "kitty")

	if !isIterm && !isWezterm && !isGhostty && !isKitty {
		return false
	}

	img, err := renderIconPNG(name)
	if err != nil {
		return false
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return false
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	if isIterm || isWezterm {
		fmt.Printf("\x1b]1337;File=inline=1;width=%dpx;height=%dpx;preserveAspectRatio=1;base64Inline=%s\a\n", size, size, encoded)
		return true
	}

	if isKitty || isGhostty {
		fmt.Printf("\x1b_Gf=100,t=d,s=%d,v=%d;%s\x1b\\\n", size, size, encoded)
		return true
	}

	return false
}

func printANSIFromPNG(name string, size int) bool {
	if size > 32 {
		size = 32
	}
	img, err := renderIconPNG(name)
	if err != nil {
		return false
	}
	
	if getIconStyle() == "half-block" {
		printANSIImage(img, size)
	} else {
		printBrailleImage(img, size)
	}
	return true
}

func renderIconPNG(name string) (image.Image, error) {
	p := getEmbeddedIconPath(name)
	if p == "" {
		return nil, fmt.Errorf("empty icon path")
	}
	data, err := iconFS.ReadFile(p)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func printTextFallback(name string) {
	// Intentionally no-op for text fallback to avoid noisy placeholders
	// like "[Codex]" in interactive TUI screens.
	_ = name
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

	return rendererAnsiAsset
}

func getEmbeddedIconPath(name string) string {
	slug := iconSlug(name)
	if slug == "" {
		return ""
	}
	return fmt.Sprintf("assets/icons/%s.png", slug)
}

func printANSIImage(img image.Image, targetWidth int) {
	b := img.Bounds()
	b = nonTransparentBounds(img, b)
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return
	}

	if targetWidth <= 0 {
		targetWidth = 20
	}
	stepX := maxInt(w/targetWidth, 1)
	stepY := stepX

	for y := b.Min.Y; y < b.Max.Y; y += stepY * 2 {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			topR, topG, topB, topA := sampleRGBA(img, x, y)
			botY := y + stepY
			botR, botG, botB, botA := uint32(0), uint32(0), uint32(0), uint32(0)
			if botY < b.Max.Y {
				botR, botG, botB, botA = sampleRGBA(img, x, botY)
			}
			if topA >= 0x1010 {
				topR, topG, topB = ansiFG, ansiFG, ansiBG
			}
			if botA >= 0x1010 {
				botR, botG, botB = ansiFG, ansiFG, ansiBG
			}

			switch {
			case topA < 0x1010 && botA < 0x1010:
				fmt.Print(" ")
			case topA >= 0x1010 && botA < 0x1010:
				fmt.Printf("\x1b[38;2;%d;%d;%dm▀\x1b[0m", topR>>8, topG>>8, topB>>8)
			case topA < 0x1010 && botA >= 0x1010:
				fmt.Printf("\x1b[38;2;%d;%d;%dm▄\x1b[0m", botR>>8, botG>>8, botB>>8)
			default:
				fmt.Printf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀\x1b[0m", topR>>8, topG>>8, topB>>8, botR>>8, botG>>8, botB>>8)
			}
		}
		fmt.Println()
	}
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
		case topA < 0x1010 && botA < 0x1010:
			out.WriteByte(' ')
		case topA >= 0x1010 && botA < 0x1010:
			out.WriteString("\x1b[38;2;214;214;232m▀\x1b[0m")
		case topA < 0x1010 && botA >= 0x1010:
			out.WriteString("\x1b[38;2;214;214;232m▄\x1b[0m")
		default:
			out.WriteString("\x1b[38;2;214;214;232;48;2;214;214;232m▀\x1b[0m")
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
			case topA < 0x1010 && botA < 0x1010:
				line.WriteByte(' ')
			case topA >= 0x1010 && botA < 0x1010:
				line.WriteString("\x1b[38;2;214;214;232m▀\x1b[0m")
			case topA < 0x1010 && botA >= 0x1010:
				line.WriteString("\x1b[38;2;214;214;232m▄\x1b[0m")
			default:
				line.WriteString("\x1b[38;2;214;214;232;48;2;214;214;232m▀\x1b[0m")
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

	size := maxInt(w, h)
	offsetX := (size - w) / 2
	offsetY := (size - h) / 2

	// We assume a single line icon is 4 dots high.
	// To match InlineIconLines(..., 3) which is 12 dots high, 
	// a single line icon is naturally 1/3 of that height.
	// But here we just want a single stable Braille line.
	step := maxInt(size/4, 1) 

	var out strings.Builder
	for j := 0; j < targetWidth; j++ {
		var offset rune
		dotMap := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}
		for dy := 0; dy < 4; dy++ {
			for dx := 0; dx < 2; dx++ {
				dotX := j*2 + dx
				dotY := dy // Single line uses top 4 dots of the square-mapped area
				
				px := b.Min.X - offsetX + dotX*step
				py := b.Min.Y - offsetY + dotY*step
				
				if px >= b.Min.X && px < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
					_, _, _, a := sampleRGBA(img, px, py)
					if a >= 0x6000 {
						offset |= dotMap[dy][dx]
					}
				}
			}
		}
		if offset == 0 {
			out.WriteByte(' ')
		} else {
			out.WriteString(fmt.Sprintf("\x1b[38;2;214;214;232m%c\x1b[0m", 0x2800+offset))
		}
	}
	return out.String()
}

func buildBrailleInlineIconLines(img image.Image, targetWidth, lines int) []string {
	// 1. Get tight bounds but calculate a square canvas to preserve aspect ratio
	b := nonTransparentBounds(img, img.Bounds())
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || lines <= 0 {
		return nil
	}

	// Determine the square size based on the larger dimension
	size := maxInt(w, h)
	offsetX := (size - w) / 2
	offsetY := (size - h) / 2

	if targetWidth <= 0 {
		targetWidth = 4
	}

	// 2. Use a consistent step for both axes to prevent distortion
	step := maxInt(size/(lines*4), 1)
	
	// Adjust targetWidth to match lines if not specified correctly for a square
	// or just use it as a boundary. 
	// Braille cell is 2 dots wide, 4 dots high.
	
	out := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		var line strings.Builder
		for j := 0; j < targetWidth; j++ {
			var offset rune
			dotMap := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}
			
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					// Map dot to image coordinates considering the centering offset
					dotX := j*2 + dx
					dotY := i*4 + dy
					
					px := b.Min.X - offsetX + dotX*step
					py := b.Min.Y - offsetY + dotY*step
					
					if px >= b.Min.X && px < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
						_, _, _, a := sampleRGBA(img, px, py)
						if a >= 0x6000 {
							offset |= dotMap[dy][dx]
						}
					}
				}
			}
			
			if offset == 0 {
				line.WriteByte(' ')
			} else {
				line.WriteString(fmt.Sprintf("\x1b[38;2;214;214;232m%c\x1b[0m", 0x2800+offset))
			}
		}
		out = append(out, line.String())
	}
	return out
}

func sampleRGBA(img image.Image, x, y int) (uint32, uint32, uint32, uint32) {
	r, g, b, a := img.At(x, y).RGBA()
	if a >= 0x1010 && a < 0xffff {
		r = r * 0xffff / a
		g = g * 0xffff / a
		b = b * 0xffff / a
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
			if a >= 0x1010 {
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

func printBrailleImage(img image.Image, targetWidth int) {
	b := img.Bounds()
	b = nonTransparentBounds(img, b)
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return
	}

	if targetWidth <= 0 {
		targetWidth = 20
	}

	stepX := maxInt(w/(targetWidth*2), 1)
	stepY := stepX

	dotMap := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

	for y := b.Min.Y; y < b.Max.Y; y += stepY * 4 {
		var line strings.Builder
		for x := b.Min.X; x < b.Max.X; x += stepX * 2 {
			var offset rune
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					px, py := x+dx*stepX, y+dy*stepY
					if px < b.Max.X && py < b.Max.Y {
						_, _, _, a := sampleRGBA(img, px, py)
						if a >= 0x6000 {
							offset |= dotMap[dy][dx]
						}
					}
				}
			}
			if offset == 0 {
				line.WriteByte(' ')
			} else {
				line.WriteString(fmt.Sprintf("\x1b[38;2;214;214;232m%c\x1b[0m", 0x2800+offset))
			}
		}
		if line.Len() > 0 {
			fmt.Println(line.String())
		}
	}
}
