package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AnnounceStateStore keeps the NIP-LSP announcement status per node in
// public.announce_state (T8-2).
type AnnounceStateStore struct {
	pool *pgxpool.Pool
}

var _ ports.AnnounceStateStore = (*AnnounceStateStore)(nil)

func NewAnnounceStateStore(pool *pgxpool.Pool) *AnnounceStateStore {
	return &AnnounceStateStore{pool: pool}
}

func (s *AnnounceStateStore) GetAnnounceState(ctx context.Context, nodeID []byte) (*core.AnnounceState, error) {
	if len(nodeID) == 0 {
		return nil, fmt.Errorf("node id required")
	}
	var status string
	var updatedAt int64
	err := s.pool.QueryRow(ctx,
		`SELECT status, updated_at FROM public.announce_state WHERE node_id = $1`,
		nodeID,
	).Scan(&status, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &core.AnnounceState{NodeID: nodeID, Status: core.AnnounceOpen}, nil
	}
	if err != nil {
		return nil, err
	}
	return stateFromRow(nodeID, core.AnnounceStatus(status), updatedAt), nil
}

// ApplyAnnounceAction locks the node's row (creating it as open first, so
// there is a row to lock), applies core.NextAnnounceStatus and writes the
// result, so two concurrent actions are applied one after the other. An
// action that leaves the status unchanged keeps the row's updated_at.
func (s *AnnounceStateStore) ApplyAnnounceAction(ctx context.Context, nodeID []byte, action core.AnnounceAction) (*core.AnnounceState, error) {
	if len(nodeID) == 0 {
		return nil, fmt.Errorf("node id required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// A placeholder row with updated_at 0 reads back as "never changed".
	if _, err := tx.Exec(ctx,
		`INSERT INTO public.announce_state (node_id, status, updated_at)
		 VALUES ($1, 'open', 0) ON CONFLICT (node_id) DO NOTHING`,
		nodeID,
	); err != nil {
		return nil, err
	}

	var current string
	var updatedAt int64
	if err := tx.QueryRow(ctx,
		`SELECT status, updated_at FROM public.announce_state WHERE node_id = $1 FOR UPDATE`,
		nodeID,
	).Scan(&current, &updatedAt); err != nil {
		return nil, err
	}

	next, err := core.NextAnnounceStatus(core.AnnounceStatus(current), action)
	if err != nil {
		return nil, err
	}
	if next != core.AnnounceStatus(current) {
		updatedAt = time.Now().UnixMicro()
		if _, err := tx.Exec(ctx,
			`UPDATE public.announce_state SET status = $2, updated_at = $3 WHERE node_id = $1`,
			nodeID, string(next), updatedAt,
		); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return stateFromRow(nodeID, next, updatedAt), nil
}

func stateFromRow(nodeID []byte, status core.AnnounceStatus, updatedAt int64) *core.AnnounceState {
	st := &core.AnnounceState{NodeID: nodeID, Status: status}
	if updatedAt != 0 {
		st.UpdatedAt = time.UnixMicro(updatedAt)
	}
	return st
}
