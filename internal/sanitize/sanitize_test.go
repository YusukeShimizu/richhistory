package sanitize_test

import (
	"testing"

	"github.com/YusukeShimizu/richhistory/internal/sanitize"
)

func TestBoundTextRemovesANSIAndTruncates(t *testing.T) {
	t.Parallel()

	input := []byte("\x1b]0;title\x07\x1b[31mhello\x1b[0m world")
	got := sanitize.BoundText(input, 5)

	if got.Text != "hello" {
		t.Fatalf("unexpected text: %q", got.Text)
	}
	if got.TotalBytes != len("hello world") {
		t.Fatalf("unexpected total bytes: %d", got.TotalBytes)
	}
	if !got.Truncated {
		t.Fatal("expected truncation")
	}
}
