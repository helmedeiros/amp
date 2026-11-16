package music

import (
	"sort"
	"strings"
	"unicode"
)

// LibraryTrack is a library track with the details search ranks and replays it by.
type LibraryTrack struct {
	Track
	ID          string
	Plays       int
	TrackNumber int
}

const (
	scoreBaseTitle       = 95
	scoreTitlePlusArtist = 92
	scoreTitlePlusAlbum  = 78
	scoreAllWords        = 35
	scoreLoose           = 20
	variantPenalty       = 3
)

var (
	titleScores  = [...]int{0, 40, 50, 58, 60, 75, 100}
	artistScores = [...]int{0, 30, 38, 42, 45, 65, 100}
	albumScores  = [...]int{0, 25, 32, 36, 40, 55, 80}
)

const (
	levelNone = iota
	levelSubstring
	levelWordStart
	levelStartsWith
	levelWord
	levelPrefix
	levelExact
)

// Search returns the tracks matching every word of query, best match first.
func Search(query string, tracks []LibraryTrack) []LibraryTrack {
	q := normalize(query)
	words := strings.Fields(q)
	if len(words) == 0 {
		return nil
	}

	type scored struct {
		LibraryTrack
		score int
	}
	var hits []scored
	for _, t := range tracks {
		if s, ok := score(q, words, t.Track); ok {
			hits = append(hits, scored{t, s})
		}
	}

	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		switch {
		case a.score != b.score:
			return a.score > b.score
		case a.Plays != b.Plays:
			return a.Plays > b.Plays
		case a.Artist != b.Artist:
			return a.Artist < b.Artist
		case a.Album != b.Album:
			return a.Album < b.Album
		default:
			return a.TrackNumber < b.TrackNumber
		}
	})

	out := make([]LibraryTrack, len(hits))
	for i, h := range hits {
		out[i] = h.LibraryTrack
	}
	return out
}

func score(q string, words []string, t Track) (int, bool) {
	name, artist, album := normalize(t.Name), normalize(t.Artist), normalize(t.Album)
	base := normalize(baseTitle(t.Name))

	hay := name + " " + artist + " " + album
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return 0, false
		}
	}

	best := max(
		titleScores[level(name, q)],
		artistScores[level(artist, q)],
		albumScores[level(album, q)],
		combo(words, base, artist, album, hay),
	)
	if base == q && name != q {
		best = max(best, scoreBaseTitle)
	}
	if isVariant(name, base, words) {
		best -= variantPenalty
	}
	return best, true
}

func level(field, q string) int {
	f, p := " "+field+" ", " "+q+" "
	switch {
	case field == q:
		return levelExact
	case strings.HasPrefix(f, p):
		return levelPrefix
	case strings.Contains(f, p):
		return levelWord
	case strings.HasPrefix(field, q):
		return levelStartsWith
	case strings.Contains(f, " "+q):
		return levelWordStart
	case strings.Contains(field, q):
		return levelSubstring
	default:
		return levelNone
	}
}

func combo(words []string, base, artist, album, hay string) int {
	titleWords := strings.Fields(base)
	title := set(titleWords)
	if len(titleWords) > 0 && containsAll(set(words), titleWords) && !containsAll(title, words) {
		rest := strings.Fields(artist)
		if containsAll(set(titleWords, rest), words) {
			return scoreTitlePlusArtist
		}
		if containsAll(set(titleWords, rest, strings.Fields(album)), words) {
			return scoreTitlePlusAlbum
		}
	}
	if containsAll(set(strings.Fields(hay)), words) {
		return scoreAllWords
	}
	return scoreLoose
}

func isVariant(name, base string, words []string) bool {
	if name == base {
		return false
	}
	query := set(words)
	for _, w := range strings.Fields(strings.TrimPrefix(name, base)) {
		if !query[w] {
			return true
		}
	}
	return false
}

func baseTitle(name string) string {
	cut := len(name)
	for _, sep := range []string{" (", " [", " - "} {
		if i := strings.Index(name, sep); i > 0 && i < cut {
			cut = i
		}
	}
	return name[:cut]
}

func set(lists ...[]string) map[string]bool {
	m := map[string]bool{}
	for _, l := range lists {
		for _, w := range l {
			m[w] = true
		}
	}
	return m
}

func containsAll(m map[string]bool, words []string) bool {
	for _, w := range words {
		if !m[w] {
			return false
		}
	}
	return true
}

func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '\'' || r == '’' || r == '`':
		case r == '&':
			b.WriteString(" and ")
		case r == 'ß':
			b.WriteString("ss")
		case r == 'æ':
			b.WriteString("ae")
		case r == 'œ':
			b.WriteString("oe")
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if f, ok := folds[r]; ok {
				r = f
			}
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var folds = func() map[rune]rune {
	m := map[rune]rune{}
	for plain, accented := range map[rune]string{
		'a': "àáâãäåāăą",
		'c': "çćĉċč",
		'd': "ďđ",
		'e': "èéêëēĕėęě",
		'g': "ĝğġģ",
		'i': "ìíîïĩīĭįı",
		'n': "ñńņňŉ",
		'o': "òóôõöøōŏő",
		'r': "ŕŗř",
		's': "śŝşš",
		't': "ţťŧ",
		'u': "ùúûüũūŭůűų",
		'y': "ýÿŷ",
		'z': "źżž",
		'l': "ĺļľŀł",
	} {
		for _, r := range accented {
			m[r] = plain
		}
	}
	return m
}()
