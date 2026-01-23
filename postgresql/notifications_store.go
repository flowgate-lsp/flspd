package postgresql

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/flowgate-lsp/flspd/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationsStore struct {
	pool *pgxpool.Pool
}

func NewNotificationsStore(pool *pgxpool.Pool) *NotificationsStore {
	return &NotificationsStore{pool: pool}
}

func (s *NotificationsStore) Register(
	ctx context.Context,
	pubkey string,
	url string,
	transport string,
) error {
	pk, err := hex.DecodeString(pubkey)
	if err != nil {
		return err
	}

	now := time.Now().UnixMicro()
	_, err = s.pool.Exec(
		ctx,
		`INSERT INTO public.notification_subscriptions (pubkey, url, transport, created_at, refreshed_at)
		 values ($1, $2, $3, $4, $5)
		 ON CONFLICT (pubkey, url) DO UPDATE SET transport = $3, refreshed_at = $5`,
		pk,
		url,
		transport,
		now,
		now,
	)

	return err
}

func (s *NotificationsStore) GetRegistrations(
	ctx context.Context,
	pubkey string,
) ([]common.NotificationRegistration, error) {
	pk, err := hex.DecodeString(pubkey)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(
		ctx,
		`SELECT url, transport 
		 FROM public.notification_subscriptions
		 WHERE pubkey = $1`,
		pk,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []common.NotificationRegistration
	for rows.Next() {
		var reg common.NotificationRegistration
		err = rows.Scan(&reg.Url, &reg.Transport)
		if err != nil {
			return nil, err
		}

		result = append(result, reg)
	}

	return result, nil
}

func (s *NotificationsStore) Unsubscribe(
	ctx context.Context,
	pubkey string,
	url string,
) error {
	pk, err := hex.DecodeString(pubkey)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(
		ctx,
		`DELETE FROM public.notification_subscriptions
		 WHERE pubkey = $1 AND url = $2`,
		pk,
		url,
	)

	return nil
}

func (s *NotificationsStore) RemoveExpired(
	ctx context.Context,
	before time.Time,
) error {
	_, err := s.pool.Exec(
		ctx,
		`DELETE FROM public.notification_subscriptions
		 WHERE refreshed_at < $1`,
		before.UnixMicro())

	return err
}
