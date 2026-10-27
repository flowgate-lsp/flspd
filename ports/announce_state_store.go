package ports

import (
	"context"
	"time"

	"github.com/flowgate-lsp/flspd/core"
)

// AnnounceStateStore keeps each node's NIP-LSP announcement status (open,
// paused, closed), set from the dashboard (T8-2) and read by the publisher
// (T8-3).
type AnnounceStateStore interface {
	// GetAnnounceState returns the node's state; a node never changed is
	// open with a zero UpdatedAt.
	GetAnnounceState(ctx context.Context, nodeID []byte) (*core.AnnounceState, error)
	// ApplyAnnounceAction applies action atomically (core.NextAnnounceStatus
	// under a row lock) and returns the new state.
	ApplyAnnounceAction(ctx context.Context, nodeID []byte, action core.AnnounceAction) (*core.AnnounceState, error)
	// MarkClosedPublished records that status closed was published, only
	// while the node is still closed; it reports whether it recorded.
	MarkClosedPublished(ctx context.Context, nodeID []byte, at time.Time) (bool, error)
}
