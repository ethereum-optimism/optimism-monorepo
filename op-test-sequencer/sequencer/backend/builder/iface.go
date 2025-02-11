package builder

import (
	"io"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/frontend"
)

// Builder provides access to block-building methods.
// Different implementations are available, e.g. for local or remote block-building.
type Builder interface {
	frontend.BuildBackend
	io.Closer
}
