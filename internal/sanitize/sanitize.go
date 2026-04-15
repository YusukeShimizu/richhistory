package sanitize

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	oscPattern = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	csiPattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
)

type BoundedText struct {
	Text        string
	TotalBytes  int
	StoredBytes int
	Truncated   bool
}

func Clean(input []byte) string {
	normalized := bytes.ToValidUTF8(input, []byte("?"))
	text := string(normalized)
	text = oscPattern.ReplaceAllString(text, "")
	text = csiPattern.ReplaceAllString(text, "")
	text = strings.Map(func(r rune) rune {
		switch {
		case r == '\n', r == '\t':
			return r
		case r < 0x20 || r == 0x7f:
			return -1
		default:
			return r
		}
	}, text)

	return text
}

func BoundText(input []byte, maxBytes int) BoundedText {
	clean := Clean(input)
	total := len([]byte(clean))
	if maxBytes <= 0 {
		return BoundedText{Text: "", TotalBytes: total}
	}
	if total <= maxBytes {
		return BoundedText{Text: clean, TotalBytes: total, StoredBytes: total}
	}

	var builder strings.Builder
	written := 0
	for _, r := range clean {
		width := utf8.RuneLen(r)
		if width < 0 {
			width = len(string(r))
		}
		if written+width > maxBytes {
			break
		}
		builder.WriteRune(r)
		written += width
	}

	return BoundedText{
		Text:        builder.String(),
		TotalBytes:  total,
		StoredBytes: written,
		Truncated:   true,
	}
}
