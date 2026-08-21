package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHandlerNameForIndex regression: listener index 10+ must produce a
// sane metrics label (previously rune('0'+i) yielded ':' and other junk).
func TestHandlerNameForIndex(t *testing.T) {
	assert.Equal(t, "handler_0", handlerNameForIndex(0))
	assert.Equal(t, "handler_9", handlerNameForIndex(9))
	assert.Equal(t, "handler_10", handlerNameForIndex(10))
	assert.Equal(t, "handler_25", handlerNameForIndex(25))
}
