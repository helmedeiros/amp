// Package soundcloud is the driven adapter that imports SoundCloud tracks using
// yt-dlp (to fetch audio) and ffmpeg (to tag it). It only reads public tracks;
// downloading is intended for a user's own uploads.
package soundcloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// Client shells out to yt-dlp and ffmpeg.
type Client struct{}

var _ port.SoundCloud = (*Client)(nil)

// New returns a Client. It does not check for the tools; call Available first.
func New() *Client { return &Client{} }

// Available reports whether the required external tools are installed, returning
// a helpful error naming what is missing.
func Available() error {
	var missing []string
	for _, tool := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing tools: %s (install with: brew install %s)",
			strings.Join(missing, ", "), strings.Join(missing, " "))
	}
	return nil
}

// List returns the tracks at url (a profile, set, or single track).
func (c *Client) List(ctx context.Context, url string) ([]music.SoundCloudTrack, error) {
	out, err := run(ctx, "yt-dlp", "--flat-playlist", "--no-warnings",
		"--print", "%(url)s\t%(title)s\t%(uploader)s", url)
	if err != nil {
		return nil, fmt.Errorf("soundcloud list: %w", err)
	}
	var tracks []music.SoundCloudTrack
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 3)
		if len(f) < 2 {
			continue
		}
		t := music.SoundCloudTrack{URL: f[0], Title: f[1]}
		if len(f) == 3 {
			t.Uploader = f[2]
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// Fetch downloads trackURL to a temporary file, tags it with the attribution,
// and writes the result into destDir, returning the final path.
func (c *Client) Fetch(ctx context.Context, trackURL, destDir string, tag music.Attribution) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "amp-sc-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	raw := filepath.Join(tmp, "raw.mp3")
	if _, err := run(ctx, "yt-dlp", "-q", "--no-warnings", "-x", "--audio-format", "mp3",
		"--audio-quality", "0", "--embed-metadata", "-o", filepath.Join(tmp, "raw.%(ext)s"), trackURL); err != nil {
		return "", fmt.Errorf("soundcloud download: %w", err)
	}
	if fi, err := os.Stat(raw); err != nil || fi.Size() == 0 {
		return "", fmt.Errorf("soundcloud download produced no audio for %s", trackURL)
	}

	out := filepath.Join(destDir, sanitize(tag.Artist+" - "+tag.Name)+".mp3")
	if _, err := run(ctx, "ffmpeg", "-y", "-loglevel", "error", "-i", raw, "-c", "copy",
		"-metadata", "title="+tag.Name,
		"-metadata", "artist="+tag.Artist,
		"-metadata", "album="+tag.Album,
		"-metadata", "album_artist="+tag.Artist,
		"-metadata", "comment="+trackURL,
		out); err != nil {
		return "", fmt.Errorf("soundcloud tag: %w", err)
	}
	return out, nil
}

// sanitize makes a title safe as a filename.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':':
			return '_'
		}
		return r
	}, s)
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%s: %s", name, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}
