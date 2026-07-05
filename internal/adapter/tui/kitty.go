package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder (Music artwork is usually JPEG)
	"image/png"
	"os"
	"strings"
)

const (
	artCols   = 16 // cell width of the artwork
	artRows   = 8  // cell height of the artwork
	artMaxDim = 256
)

// kittySupported reports whether the terminal understands the Kitty graphics
// protocol. AMP_ART=1 forces it on, AMP_ART=0 off.
func kittySupported() bool {
	switch os.Getenv("AMP_ART") {
	case "1":
		return true
	case "0":
		return false
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" || strings.Contains(os.Getenv("TERM"), "kitty") {
		return true
	}
	return os.Getenv("TERM_PROGRAM") == "ghostty" || os.Getenv("GHOSTTY_RESOURCES_DIR") != ""
}

// encodeKittyImage decodes image bytes, downscales, and returns a Kitty escape
// that displays it scaled into cols x rows cells at the cursor.
func encodeKittyImage(data []byte, cols, rows int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode artwork: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, downscale(img, artMaxDim)); err != nil {
		return "", fmt.Errorf("encode artwork: %w", err)
	}

	return kittyEscape(base64.StdEncoding.EncodeToString(buf.Bytes()), cols, rows), nil
}

// kittyEscape builds the (chunked) Kitty escape for a base64 PNG payload.
func kittyEscape(b64 string, cols, rows int) string {
	const chunk = 4096
	var b strings.Builder
	first := true

	for len(b64) > 0 {
		n := min(chunk, len(b64))
		part := b64[:n]
		b64 = b64[n:]
		more := 0
		if len(b64) > 0 {
			more = 1
		}

		b.WriteString("\x1b_G")
		if first {
			fmt.Fprintf(&b, "a=T,f=100,c=%d,r=%d,m=%d", cols, rows, more)
			first = false
		} else {
			fmt.Fprintf(&b, "m=%d", more)
		}
		b.WriteString(";")
		b.WriteString(part)
		b.WriteString("\x1b\\")
	}
	return b.String()
}

// downscale shrinks src so its longest side is at most max, using
// nearest-neighbour sampling (no external dependency).
func downscale(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return src
	}

	scale := float64(maxDim) / float64(max(w, h))
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + int(float64(y)/scale)
		for x := 0; x < nw; x++ {
			sx := b.Min.X + int(float64(x)/scale)
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
