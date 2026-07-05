package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tinyPNG returns a small solid-color PNG for tests.
func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestKittyEscapeChunks(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("A", 10000) // > 2 chunks of 4096
	esc := kittyEscape(payload, 16, 8)

	assert.True(t, strings.HasPrefix(esc, "\x1b_Ga=T,f=100,c=16,r=8,m=1;"), "first chunk sets params + more")
	assert.Contains(t, esc, "\x1b_Gm=1;", "middle chunk continues")
	assert.True(t, strings.HasSuffix(esc, "\x1b\\"), "ends with the escape terminator")
	// last chunk marks completion
	assert.Contains(t, esc, "m=0;")
}

func TestEncodeKittyImage(t *testing.T) {
	t.Parallel()

	esc, err := encodeKittyImage(tinyPNG(t, 300, 300), 16, 8)

	require.NoError(t, err)
	assert.Contains(t, esc, "\x1b_Ga=T,f=100,c=16,r=8")
}

func TestEncodeKittyImageRejectsGarbage(t *testing.T) {
	t.Parallel()

	_, err := encodeKittyImage([]byte("not an image"), 16, 8)
	require.Error(t, err)
}

func TestKittySupported(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("GHOSTTY_RESOURCES_DIR", "")

	t.Run("forced on", func(t *testing.T) {
		t.Setenv("AMP_ART", "1")
		assert.True(t, kittySupported())
	})
	t.Run("forced off", func(t *testing.T) {
		t.Setenv("AMP_ART", "0")
		t.Setenv("KITTY_WINDOW_ID", "1")
		assert.False(t, kittySupported())
	})
	t.Run("ghostty", func(t *testing.T) {
		t.Setenv("AMP_ART", "")
		t.Setenv("TERM_PROGRAM", "ghostty")
		assert.True(t, kittySupported())
	})
	t.Run("plain terminal", func(t *testing.T) {
		t.Setenv("AMP_ART", "")
		t.Setenv("TERM_PROGRAM", "")
		assert.False(t, kittySupported())
	})
}
