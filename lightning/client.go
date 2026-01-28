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

type HTLC struct {
	Incoming         bool
	AmountMloki      uint64
	HashLock         []byte
	ExpirationHeight uint32
	HtlcIndex        uint64
}

type Channel struct {
	AliasScid     *ShortChannelID
	ConfirmedScid *ShortChannelID
	ChannelPoint  *wire.OutPoint
	PeerId        []byte
	PendingHtlcs  []HTLC
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
	GetBlockHeight() (uint32, error)
	QueryRoutes(destination []byte, amountMloki uint64, outgoingChanId uint64) ([]*Route, error)
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

type Route struct {
	TotalTimeLock uint32
	TotalFees     int64
	TotalAmt      int64
	Hops          []*Hop
}

type Hop struct {
	ChanId           uint64
	ChanCapacity     int64
	AmtToForward     int64
	Fee              int64
	Expiry           uint32
	AmtToForwardMsat int64
	FeeMsat          int64
	PubKey           string
	CustomRecords    map[uint64][]byte
}
