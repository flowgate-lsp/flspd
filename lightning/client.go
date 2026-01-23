package lightning

import (
	"context"
	"time"

	"github.com/flokiorg/go-flokicoin/wire"
)

type GetInfoResult struct {
	Alias  string
	Pubkey string
}

type GetChannelResult struct {
	AliasScid        *ShortChannelID
	ConfirmedScid    *ShortChannelID
	HtlcMinimumMloki uint64
}

type RoutingPolicy struct {
	Ppm           uint32
	BaseMloki     uint64
	TimeLockDelta uint32
}

type OpenChannelRequest struct {
	Destination     []byte
	CapacityLoki    uint64
	MinConfs        *uint32
	FeeLokiPerVByte *float64
	TargetConf      *uint32
	RoutingPolicy   *RoutingPolicy
	ZeroConf        bool
	PushLoki        uint64
	AnnounceChannel bool
}

type Channel struct {
	AliasScid     *ShortChannelID
	ConfirmedScid *ShortChannelID
	ChannelPoint  *wire.OutPoint
	PeerId        []byte
}

type Client interface {
	GetInfo() (*GetInfoResult, error)
	IsConnected(destination []byte) (bool, error)
	OpenChannel(req *OpenChannelRequest) (*wire.OutPoint, error)
	GetChannel(peerID []byte, channelPoint wire.OutPoint) (*GetChannelResult, error)
	GetPeerId(scid *ShortChannelID) ([]byte, error)
	GetClosedChannels(nodeID string, channelPoints map[string]uint64) (map[string]uint64, error)
	WaitOnline(peerID []byte, deadline time.Time) error
	WaitChannelActive(peerID []byte, deadline time.Time) error
	ListChannels() ([]*Channel, error)
	SubscribeInvoices() (<-chan *InvoiceUpdate, <-chan error, error)
	WalletBalance(ctx context.Context) (*WalletBalanceResult, error)
}

type WalletBalanceResult struct {
	TotalBalance       int64
	ConfirmedBalance   int64
	UnconfirmedBalance int64
}

// InvoiceState represents the state of an invoice
type InvoiceState int

const (
	InvoiceStateOpen InvoiceState = iota
	InvoiceStateSettled
	InvoiceStateCanceled
	InvoiceStateAccepted
)

// InvoiceUpdate represents an invoice update event from LND
type InvoiceUpdate struct {
	State          InvoiceState
	PaymentRequest string
	AmountMloki    uint64
	SettleDate     time.Time
}
