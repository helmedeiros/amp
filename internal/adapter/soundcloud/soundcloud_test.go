package soundcloud

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmedeiros/amp/internal/port"
)

func TestSanitizeMakesSafeFilenames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Platoon - E.U.A.", sanitize("Platoon - E.U.A."))
	assert.Equal(t, "AC_DC - T.N.T", sanitize("AC/DC - T.N.T"))
	assert.Equal(t, "a_b_c", sanitize("a:b/c"))
}

func TestClientSatisfiesPort(t *testing.T) {
	t.Parallel()

	var _ port.SoundCloud = New()
}
