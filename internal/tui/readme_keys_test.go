package tui

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// README のキー表は、書いた時点のコードと合っていても、キーを動かせば黙って
// 古くなる。表の1文字キーが keys.go のどこかで受けられていることを見る。
func TestReadmeKeysExistInTheKeyHandler(t *testing.T) {
	t.Parallel()
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	keys, err := os.ReadFile("keys.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(keys)
	rows := regexp.MustCompile("(?m)^\\| (`[^|]+) \\| [^|]+ \\|$").FindAllStringSubmatch(string(readme), -1)
	if len(rows) < 15 {
		t.Fatalf("README のキー表が %d 行しか読めない", len(rows))
	}
	single := regexp.MustCompile("`(?:shift\\+)?([a-zA-Z])`")
	seen := 0
	for _, row := range rows {
		for _, m := range single.FindAllStringSubmatch(row[1], -1) {
			k := strings.ToLower(m[1])
			seen++
			if !strings.Contains(src, "case '"+k+"'") &&
				!strings.Contains(src, "'"+k+"',") &&
				!strings.Contains(src, ", '"+k+"'") &&
				!strings.Contains(src, "Code == '"+k+"'") {
				t.Errorf("README は %q を挙げるが keys.go に受け口が無い", m[1])
			}
		}
	}
	if seen < 15 {
		t.Fatalf("表から %d 個しかキーを読めていない", seen)
	}
}
