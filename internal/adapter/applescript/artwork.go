package applescript

import (
	"fmt"
	"strings"
)

// saveArtworkScript writes the current track's album artwork to path. It errors
// (non-zero exit) when nothing is playing or the track has no artwork.
func saveArtworkScript(path string) string {
	return fmt.Sprintf(`tell application "Music"
  if player state is stopped then error "no current track"
  set theData to data of artwork 1 of current track
end tell
set f to open for access (POSIX file "%s") with write permission
set eof f to 0
write theData to f
close access f`, escapeAppleScript(path))
}

// escapeAppleScript escapes a string for embedding in an AppleScript literal.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
