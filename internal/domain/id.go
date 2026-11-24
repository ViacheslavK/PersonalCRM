package domain

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	idMu      sync.Mutex
	idEntropy = ulid.Monotonic(rand.Reader, 0)
)

// NewID generates a time-sortable, globally unique ULID string.
func NewID() string {
	idMu.Lock()
	defer idMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), idEntropy).String()
}
