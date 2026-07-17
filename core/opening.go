package core

import "time"

type OpeningFeeParamsSetting struct {
	Validity time.Duration
	Params   *OpeningFeeParams
}

type OpeningFeeParams struct {
	MinFeeMloki          uint64 `json:"min_mloki,string"`
	Proportional         uint32 `json:"proportional"`
	ValidUntil           string `json:"valid_until"`
	MinLifetime          uint32 `json:"max_idle_time"`
	MaxClientToSelfDelay uint32 `json:"max_client_to_self_delay"`
	MinPaymentSizeMloki  uint64 `json:"min_payment_size_mloki,string"`
	MaxPaymentSizeMloki  uint64 `json:"max_payment_size_mloki,string"`
	Promise              string `json:"promise"`
}

type Lsps2BuyRegistration struct {
	Id                         string
	LspId                      string
	PeerId                     string
	Scid                       uint64
	Mode                       uint16 // 0 or 1
	PaymentSizeMloki           *uint64
	ParamsMinFeeMloki          uint64
	ParamsProportional         uint32
	ParamsValidUntil           string
	ParamsMinLifetime          uint32
	ParamsMaxClientToSelfDelay uint32
	ParamsPromise              string
	Token                      string
}

type Lsps1Order struct {
	OrderId                      string    `json:"order_id"`
	PeerId                       string    `json:"peer_id"`
	LspBalanceLoki               int64     `json:"lsp_balance_loki"`
	ClientBalanceLoki            int64     `json:"client_balance_loki"`
	FundingConfirmsWithinBlocks  int       `json:"funding_confirms_within_blocks"`
	RequiredChannelConfirmations int       `json:"required_channel_confirmations"`
	ChannelExpiryBlocks          int       `json:"channel_expiry_blocks"`
	Token                        string    `json:"token"`
	AnnounceChannel              bool      `json:"announce_channel"`
	CreatedAt                    time.Time `json:"created_at"`
	OrderState                   string    `json:"state"`
	PaymentState                 string    `json:"payment_state"`
	PaymentInvoice               string    `json:"payment_invoice"`
	// PaymentPreimage is the LSP-generated hex-encoded preimage backing a
	// hold invoice (PaymentInvoice's payment hash is sha256 of this).
	// Never serialize this to any external response -- it's the secret
	// that authorizes settling the invoice, so json:"-" keeps it out of
	// any accidental json.Marshal(order) (e.g. an admin/dashboard
	// listing). Nil for orders created before hold invoices, or for
	// orders that still use the legacy AddInvoice flow.
	PaymentPreimage      *string `json:"-"`
	FundingTxid          *string `json:"funding_txid"`
	FundingOutpoint      *int    `json:"funding_outpoint"`
	RefundOnchainAddress string  `json:"refund_onchain_address"`
	OrderTotalLoki       int64   `json:"amount"` // Mapped to 'amount' for frontend simplicity
	FeeTotalLoki         int64   `json:"fee"`    // Mapped to 'fee' for frontend simplicity
	PeerAlias            string  `json:"peer_alias"`
	PeerColor            string  `json:"peer_color"`
}

type Lsps5Webhook struct {
	ClientNodeId string
	AppName      string
	Url          string
	Transport    string // "push" or "nostr"
	CreatedAt    time.Time
}
