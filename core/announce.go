package core

import (
	"fmt"
	"time"
)

// AnnounceStatus is a node's NIP-LSP announcement `status` (T8-2): runtime
// state set from the dashboard and kept in the database, not in nodes.yaml.
type AnnounceStatus string

const (
	// AnnounceOpen: the LSP takes orders. The state of a node never changed.
	AnnounceOpen AnnounceStatus = "open"
	// AnnouncePaused: no new orders for now (dashboard Pause).
	AnnouncePaused AnnounceStatus = "paused"
	// AnnounceClosed: the LSP has stopped selling (dashboard Unlist). The
	// publisher announces it once, then stops publishing.
	AnnounceClosed AnnounceStatus = "closed"
)

// Valid reports whether s is one of the three NIP-LSP statuses.
func (s AnnounceStatus) Valid() bool {
	return s == AnnounceOpen || s == AnnouncePaused || s == AnnounceClosed
}

// AnnounceAction is a dashboard action on the announcement status.
type AnnounceAction string

const (
	AnnouncePause  AnnounceAction = "pause"
	AnnounceResume AnnounceAction = "resume"
	AnnounceUnlist AnnounceAction = "unlist"
)

// ErrAnnounceTransition is returned for an action the current status does
// not allow.
var ErrAnnounceTransition = fmt.Errorf("announce status transition not allowed")

// NextAnnounceStatus applies action to current:
//
//   - pause:  open → paused (paused stays paused); refused while closed,
//     since an unlisted LSP is listed again with resume, not pause.
//   - resume: paused or closed → open (open stays open). Resuming a closed
//     node lists it again.
//   - unlist: any → closed.
func NextAnnounceStatus(current AnnounceStatus, action AnnounceAction) (AnnounceStatus, error) {
	if !current.Valid() {
		return "", fmt.Errorf("unknown announce status %q", current)
	}
	switch action {
	case AnnouncePause:
		if current == AnnounceClosed {
			return "", fmt.Errorf("%w: the node is unlisted; resume lists it again", ErrAnnounceTransition)
		}
		return AnnouncePaused, nil
	case AnnounceResume:
		return AnnounceOpen, nil
	case AnnounceUnlist:
		return AnnounceClosed, nil
	default:
		return "", fmt.Errorf("unknown announce action %q (want pause, resume or unlist)", action)
	}
}

// AnnounceState is a node's stored announcement status.
type AnnounceState struct {
	NodeID []byte
	Status AnnounceStatus
	// UpdatedAt is when Status last changed; zero for a node never changed.
	UpdatedAt time.Time
}
