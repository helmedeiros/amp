package applescript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveArtworkScript(t *testing.T) {
	t.Parallel()

	script := saveArtworkScript(`/tmp/my "art".jpg`)

	assert.Contains(t, script, "data of artwork 1 of current track")
	assert.Contains(t, script, `POSIX file "/tmp/my \"art\".jpg"`, "path is escaped")
}

func TestPlayerSaveArtwork(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{}
	require.NoError(t, newPlayer(fake).SaveArtwork(context.Background(), "/tmp/cover.jpg"))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, appleScript, fake.calls[0].lang)
	assert.Contains(t, fake.calls[0].script, "/tmp/cover.jpg")
}
