package config

type NodeConfig struct {
	// Name of the LSP. If empty, the node's alias will be taken instead. The
	// name is returned in the ChannelInformation api endpoint as a friendly
	// name.
	Name string `json:"name,omitempty"`

	// The public key of the lightning node. If empty, the node's pubkey will be
	// taken instead. The pubkey is returned in the ChannelInformation api
	// endpoint, s
	NodePubkey string `json:"nodePubkey,omitempty"`

	// Hex encoded private key of the LSP. This is used to decrypt traffic from
	// clients. Note this is not the private key of the node.
	FlspdPrivateKey string `json:"flspdPrivateKey"`

	// Tokens used to authenticate to flspd. These tokens must be unique for each
	// configured node, so it's obvious which node an rpc call is meant for.
	Tokens []string `json:"tokens"`

	// The network location of the lightning node, e.g. `12.34.56.78:9012` or
	// `localhost:10011`. The host is returned in the ChannelInformation api
	// endpoint so clients can connect to the node.
	Host string `json:"host"`

	// Public channel amount is a reserved amount for public channels. If a
	// zero conf channel is opened, it will never have this exact amount.
	PublicChannelAmount int64 `json:"publicChannelAmount,string,omitempty"`

	// Number of blocks after which an opened channel is considered confirmed.
	TargetConf uint32 `json:"targetConf,string"`

	// Minimum number of confirmations inputs for zero conf channel opens should
	// have.
	MinConfs *uint32 `json:"minConfs,string"`

	// Smallest htlc amount routed over channels a channel. It is configured on
	// the node itself, but this value is returned in the ChannelInformation rpc.
	MinHtlcMloki uint64 `json:"minHtlcMloki,string"`

	// The base fee for routing payments over the channel. It is configured on
	// the node itself, but this value is returned in the ChannelInformation rpc.
	BaseFeeMloki uint64 `json:"baseFeeMloki,string"`

	// The fee rate for routing payments over the channel. It is configured on
	// the node itself, but this value is returned in the ChannelInformation rpc.
	FeeRate float64 `json:"feeRate,string"`

	// Minimum timelock delta required for opening a zero conf channel.
	TimeLockDelta uint32 `json:"timeLockDelta,string"`

	// Fee for opening a zero conf channel in loki per 10000 loki based
	// on the incoming payment amount.
	// This field is for legacy support for clients that don't support
	// OpeningFeeParams.
	ChannelFeePermyriad int64 `json:"channelFeePermyriad,string"`

	// Minimum fee for opening a zero conf channel in milliloki. If the fee
	// using ChannelFeePermyriad is less than this amount, this amount is the
	// actual fee to be paid.
	// This field is for legacy support for clients that don't support
	// OpeningFeeParams.
	ChannelMinimumFeeMloki int64 `json:"channelMinimumFeeMloki,string"`

	// Channel capacity that is added on top of the incoming payment amount
	// when a new zero conf channel is opened. In loki.
	AdditionalChannelCapacity int64 `json:"additionalChannelCapacity,string"`

	// The channel can be closed if not used this duration in seconds. This is
	// not enforced by flspd, but gives an indication to clients.
	// This field is for legacy support for clients that don't support
	// OpeningFeeParams.
	MaxInactiveDuration uint64 `json:"maxInactiveDuration,string"`

	// The maximum time to hold a htlc after sending a notification when the
	// peer is offline. Defaults to 1m.
	NotificationTimeout string `json:"notificationTimeout,string"`

	// Minimum payment size in mloki for JIT channels.
	MinPaymentSizeMloki uint64 `json:"minPaymentSizeMloki,string,omitempty"`

	// Maximum payment size in mloki for JIT channels.
	MaxPaymentSizeMloki uint64 `json:"maxPaymentSizeMloki,string,omitempty"`

	// If true, enables JIT channels to override MinConfs to 0 (implicit zero conf).
	JitEnabled bool `json:"jitEnabled,omitempty"`

	// Set this field to connect to an LND node.
	Lnd *LndConfig `json:"flnd,omitempty"`
}

type LndConfig struct {
	// Address to the grpc api.
	Address string `json:"address"`

	// tls cert for the grpc api.
	Cert string `json:"cert"`

	// macaroon to use.
	Macaroon string `json:"macaroon"`

	// DisableTls disables TLS verification for the LND connection.
	// This is useful for testing or internal networks (e.g. docker).
	DisableTls bool `json:"disableTls,omitempty"`
}
