package notifications

import (
	"context"
	"time"

	"github.com/flowgate-lsp/flspd/common"
)

type Store interface {
	Register(ctx context.Context, pubkey string, url string, transport string) error
	GetRegistrations(ctx context.Context, pubkey string) ([]common.NotificationRegistration, error)
	Unsubscribe(ctx context.Context, pubkey string, url string) error
	RemoveExpired(ctx context.Context, before time.Time) error
}
