package config

type NodeConfig struct {
	// Name of the LSP. If empty, the node's alias will be taken instead. The
	// name is returned in the ChannelInformation api endpoint as a friendly
	// name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// The public key of the lightning node. If empty, the node's pubkey will be
	// taken instead. The pubkey is returned in the ChannelInformation api
	// endpoint, s
	NodePubkey string `json:"nodePubkey,omitempty" yaml:"nodePubkey,omitempty"`

	// Hex encoded private key of the LSP. This is used to decrypt traffic from
	// clients. Note this is not the private key of the node.
	FlspdPrivateKey string `json:"flspdPrivateKey" yaml:"flspdPrivateKey"`

	// Tokens used to authenticate to flspd. These tokens must be unique for each
	// configured node, so it's obvious which node an rpc call is meant for.
	Tokens []string `json:"tokens" yaml:"tokens"`

	// The network location of the lightning node, e.g. `12.34.56.78:9012` or
	// `localhost:10011`. The host is returned in the ChannelInformation api
	// endpoint so clients can connect to the node.
	Host string `json:"host" yaml:"host"`

	// Public channel amount is a reserved amount for public channels. If a
	// zero conf channel is opened, it will never have this exact amount.
	PublicChannelAmount int64 `json:"publicChannelAmount,string,omitempty" yaml:"publicChannelAmount,omitempty"`

	// Number of blocks after which an opened channel is considered confirmed.
	TargetConf uint32 `json:"targetConf,string" yaml:"targetConf"`

	// Minimum number of confirmations inputs for zero conf channel opens should
	// have.
	MinConfs *uint32 `json:"minConfs,string" yaml:"minConfs"`

	// Smallest htlc amount routed over channels a channel. It is configured on
	// the node itself, but this value is returned in the ChannelInformation rpc.
	MinHtlcMloki uint64 `json:"minHtlcMloki,string" yaml:"minHtlcMloki"`

	// The base fee for routing payments over the channel. It is configured on
	// the node itself, but this value is returned in the ChannelInformation rpc.
	BaseFeeMloki uint64 `json:"baseFeeMloki,string" yaml:"baseFeeMloki"`

	// The fee rate for routing payments over the channel. It is configured on
	// the node itself, but this value is returned in the ChannelInformation rpc.
	FeeRate float64 `json:"feeRate,string" yaml:"feeRate"`

	// Minimum timelock delta required for opening a zero conf channel.
	TimeLockDelta uint32 `json:"timeLockDelta,string" yaml:"timeLockDelta"`

	// Fee for opening a zero conf channel in loki per 10000 loki based
	// on the incoming payment amount.
	// This field is for legacy support for clients that don't support
	// OpeningFeeParams.
	ChannelFeePermyriad int64 `json:"channelFeePermyriad,string" yaml:"channelFeePermyriad"`

	// Minimum fee for opening a zero conf channel in milliloki. If the fee
	// using ChannelFeePermyriad is less than this amount, this amount is the
	// actual fee to be paid.
	// This field is for legacy support for clients that don't support
	// OpeningFeeParams.
	ChannelMinimumFeeMloki int64 `json:"channelMinimumFeeMloki,string" yaml:"channelMinimumFeeMloki"`

	// Channel capacity that is added on top of the incoming payment amount
	// when a new zero conf channel is opened. In loki.
	AdditionalChannelCapacity int64 `json:"additionalChannelCapacity,string" yaml:"additionalChannelCapacity"`

	// The channel can be closed if not used this duration in seconds. This is
	// not enforced by flspd, but gives an indication to clients.
	// This field is for legacy support for clients that don't support
	// OpeningFeeParams.
	MaxInactiveDuration uint64 `json:"maxInactiveDuration,string" yaml:"maxInactiveDuration"`

	// The maximum time to hold a htlc after sending a notification when the
	// peer is offline. Defaults to 1m.
	NotificationTimeout string `json:"notificationTimeout,string" yaml:"notificationTimeout"`

	// Minimum payment size in mloki for JIT channels.
	MinPaymentSizeMloki uint64 `json:"minPaymentSizeMloki,string,omitempty" yaml:"minPaymentSizeMloki,omitempty"`

	// Maximum payment size in mloki for JIT channels.
	MaxPaymentSizeMloki uint64 `json:"maxPaymentSizeMloki,string,omitempty" yaml:"maxPaymentSizeMloki,omitempty"`

	// Minimum channel capacity in loki. Must be >= 20000 (LND minimum).
	MinChannelCapacityLoki uint64 `json:"minChannelCapacityLoki,string,omitempty" yaml:"minChannelCapacityLoki,omitempty"`

	// If true, enables JIT channels to override MinConfs to 0 (implicit zero conf).
	JitEnabled bool `json:"jitEnabled,omitempty" yaml:"jitEnabled,omitempty"`

	// JIT Channels Configuration
	// Maximum ClientToSelfDelay for JIT channel opening fee params.
	JitMaxClientToSelfDelay uint32 `json:"jitMaxClientToSelfDelay,string,omitempty" yaml:"jitMaxClientToSelfDelay,omitempty"`

	// Validity duration for JIT channel opening fee params in seconds.
	JitValidUntilSeconds uint32 `json:"jitValidUntilSeconds,string,omitempty" yaml:"jitValidUntilSeconds,omitempty"`

	// LSPS1 Configuration
	// Minimum required channel confirmations for LSPS1 orders.
	Lsps1MinRequiredChannelConfirmations uint16 `json:"lsps1MinRequiredChannelConfirmations,string,omitempty" yaml:"lsps1MinRequiredChannelConfirmations,omitempty"`

	// Minimum funding confirms within blocks for LSPS1 orders.
	Lsps1MinFundingConfirmsWithinBlocks uint16 `json:"lsps1MinFundingConfirmsWithinBlocks,string,omitempty" yaml:"lsps1MinFundingConfirmsWithinBlocks,omitempty"`

	// Maximum channel expiry blocks for LSPS1 orders.
	Lsps1MaxChannelExpiryBlocks uint32 `json:"lsps1MaxChannelExpiryBlocks,string,omitempty" yaml:"lsps1MaxChannelExpiryBlocks,omitempty"`

	// Duration in seconds before an LSPS1 order expires if not funded.
	Lsps1OrderExpirySeconds uint32 `json:"lsps1OrderExpirySeconds,string,omitempty" yaml:"lsps1OrderExpirySeconds,omitempty"`

	// Maximum initial client balance in Loki for LSPS1 orders.
	Lsps1MaxClientBalanceLoki uint64 `json:"lsps1MaxClientBalanceLoki,string,omitempty" yaml:"lsps1MaxClientBalanceLoki,omitempty"`

	// Maximum initial LSP balance in Loki for LSPS1 orders.
	Lsps1MaxLspBalanceLoki uint64 `json:"lsps1MaxLspBalanceLoki,string,omitempty" yaml:"lsps1MaxLspBalanceLoki,omitempty"`

	// Set this field to connect to an LND node.
	Lnd *LndConfig `json:"flnd,omitempty" yaml:"flnd,omitempty"`
}

type LndConfig struct {
	// Address to the grpc api.
	Address string `json:"address" yaml:"address"`

	// tls cert for the grpc api.
	Cert string `json:"cert" yaml:"cert"`

	// macaroon to use.
	Macaroon string `json:"macaroon" yaml:"macaroon"`

	// DisableTls disables TLS verification for the LND connection.
	// This is useful for testing or internal networks (e.g. docker).
	DisableTls bool `json:"disableTls,omitempty" yaml:"disableTls,omitempty"`
}
