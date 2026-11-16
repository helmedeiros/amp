package music_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmedeiros/amp/internal/music"
)

func lt(name, artist, album string, plays int) music.LibraryTrack {
	return music.LibraryTrack{
		Track: music.Track{Name: name, Artist: artist, Album: album},
		ID:    name + "/" + artist + "/" + album,
		Plays: plays,
	}
}

func names(ts []music.LibraryTrack) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name + " · " + t.Artist
	}
	return out
}

func TestSearchRanksExactTitleFirst(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Lovely Day", "Bill Withers", "Menagerie", 50),
		lt("Crazy in Love", "Beyoncé", "Dangerously in Love", 40),
		lt("Love", "Kendrick Lamar", "DAMN.", 1),
		lt("Love Story", "Taylor Swift", "Fearless", 30),
	}

	got := music.Search("love", lib)

	assert.Equal(t, []string{
		"Love · Kendrick Lamar",
		"Love Story · Taylor Swift",
		"Crazy in Love · Beyoncé",
		"Lovely Day · Bill Withers",
	}, names(got))
}

func TestSearchPrefersOriginalOverVersions(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Yesterday (Live)", "The Beatles", "Live at the BBC", 9),
		lt("Yesterday - Remastered 2009", "The Beatles", "Help!", 9),
		lt("Yesterday", "The Beatles", "Help!", 2),
	}

	got := music.Search("yesterday", lib)

	assert.Equal(t, "Yesterday", got[0].Name)
}

func TestSearchVersionAskedForWins(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Yesterday", "The Beatles", "Help!", 9),
		lt("Yesterday (Live)", "The Beatles", "Live at the BBC", 0),
	}

	got := music.Search("yesterday live", lib)

	assert.Equal(t, "Yesterday (Live)", got[0].Name)
}

func TestSearchTitlePlusArtistBeatsLooserMatches(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Help!", "The Beatles", "Help!", 99),
		lt("Yesterday Once More", "Carpenters", "Now & Then", 99),
		lt("Yesterday", "The Beatles", "Help!", 0),
	}

	got := music.Search("yesterday beatles", lib)

	assert.Equal(t, []string{"Yesterday · The Beatles"}, names(got))
}

func TestSearchRepeatedTitleWordIsNotATitlePlusArtistMatch(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Love, Love, Love", "Donny Hathaway", "Extension of a Man", 99),
		lt("Love (Remastered 2012)", "The Smashing Pumpkins", "Mellon Collie", 0),
	}

	got := music.Search("love", lib)

	assert.Equal(t, "The Smashing Pumpkins", got[0].Artist)
}

func TestSearchArtistOutranksIncidentalTitleWord(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Daft Punk Is Playing at My House", "LCD Soundsystem", "LCD Soundsystem", 3),
		lt("One More Time", "Daft Punk", "Discovery", 1),
	}

	got := music.Search("daft punk", lib)

	assert.Equal(t, "One More Time", got[0].Name)
}

func TestSearchExactArtistBeatsSongNamedAfterIt(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Oasis", "The Kooks", "10 Tracks to Echo in the Dark", 1),
		lt("Wonderwall", "Oasis", "(What's the Story) Morning Glory?", 30),
	}

	got := music.Search("oasis", lib)

	assert.Equal(t, "Wonderwall", got[0].Name)
}

func TestSearchPartialLastWordStillMatchesTitleStart(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Stella Was a Diver and She Was Always Down", "Interpol", "Turn On The Bright Lights", 9),
		lt("And She Was", "Talking Heads", "Little Creatures", 0),
	}

	got := music.Search("and she wa", lib)

	assert.Equal(t, "And She Was", got[0].Name)
}

func TestSearchExactAlbumBeatsTitlePlusAlbumWord(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("You Don't Love Me", "The Kooks", "Inside In / Inside Out - EP", 9),
		lt("You Don't Love Me", "The Kooks", "You Don't Love Me - EP", 0),
	}

	got := music.Search("you don't love me - ep", lib)

	assert.Equal(t, "You Don't Love Me - EP", got[0].Album)
}

func TestSearchBreaksTiesByPlays(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Hurt", "Nine Inch Nails", "The Downward Spiral", 2),
		lt("Hurt", "Johnny Cash", "American IV", 40),
	}

	got := music.Search("hurt", lib)

	assert.Equal(t, "Johnny Cash", got[0].Artist)
}

func TestSearchKeepsAlbumOrderForAlbumMatches(t *testing.T) {
	t.Parallel()

	a := lt("Aerodynamic", "Daft Punk", "Discovery", 0)
	a.TrackNumber = 3
	b := lt("One More Time", "Daft Punk", "Discovery", 0)
	b.TrackNumber = 1

	got := music.Search("discovery", []music.LibraryTrack{a, b})

	assert.Equal(t, []string{"One More Time · Daft Punk", "Aerodynamic · Daft Punk"}, names(got))
}

func TestSearchIgnoresCaseAccentsAndPunctuation(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("Jóga", "Björk", "Homogenic", 0),
		lt("The Sound of Silence", "Simon & Garfunkel", "Sounds of Silence", 0),
		lt("Don't Stop Me Now", "Queen", "Jazz", 0),
	}

	assert.Len(t, music.Search("BJORK joga", lib), 1)
	assert.Len(t, music.Search("simon and garfunkel", lib), 1)
	assert.Len(t, music.Search("dont stop", lib), 1)
}

func TestSearchRequiresEveryWord(t *testing.T) {
	t.Parallel()

	lib := []music.LibraryTrack{
		lt("One More Time", "Daft Punk", "Discovery", 0),
		lt("One", "U2", "Achtung Baby", 0),
	}

	assert.Equal(t, []string{"One More Time · Daft Punk"}, names(music.Search("one daft", lib)))
	assert.Empty(t, music.Search("one metallica", lib))
	assert.Empty(t, music.Search("   ", lib))
}
