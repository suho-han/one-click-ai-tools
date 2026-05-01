package ui

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// PrintIconFromURL 터미널이 이미지 출력을 지원하는 경우 Lobe Icons의 PNG를 출력합니다.
func PrintIconFromURL(url string, size int) {
	// 터미널 프로그램 확인
	term := os.Getenv("TERM_PROGRAM")
	isIterm := term == "iTerm.app"
	isWezterm := term == "WezTerm"
	
	// 지원하지 않는 터미널이면 무시
	if !isIterm && !isWezterm && os.Getenv("TERMINAL_EMULATOR") != "kitty" {
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if isIterm || isWezterm {
		// iTerm2 Inline Image Protocol
		fmt.Printf("\x1b]1337;File=inline=1;width=%dpx;height=%dpx;preserveAspectRatio=1;base64Inline=%s\a\n", 
			size, size, base64.StdEncoding.EncodeToString(data))
	}
}

// GetLobeIconURL Lobe Icons CDN URL을 반환합니다.
func GetLobeIconURL(name string) string {
	// Lobe Icons는 kebab-case를 주로 사용하므로 변환이 필요할 수 있습니다.
	kebabName := strings.ToLower(name)
	return fmt.Sprintf("https://registry.npmmirror.com/@lobehub/icons-static-png/latest/files/light/%s.png", kebabName)
}
