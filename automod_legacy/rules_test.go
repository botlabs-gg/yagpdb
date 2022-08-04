package automod_legacy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCachedInvites(t *testing.T) {
	invitesCache.set("foo", 123)
	i, ok := invitesCache.get("foo")
	assert.True(t, ok)
	assert.EqualValues(t, 123, i.guildID)

	time.Sleep(50 * time.Millisecond)
	invitesCache.tick(time.Millisecond)
	_, ok = invitesCache.get("foo")
	assert.False(t, ok)
}
