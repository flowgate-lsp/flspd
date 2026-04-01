package common

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
	FundingTxid                  *string   `json:"funding_txid"`
	FundingOutpoint              *int      `json:"funding_outpoint"`
	RefundOnchainAddress         string    `json:"refund_onchain_address"`
	OrderTotalLoki               int64     `json:"amount"` // Mapped to 'amount' for frontend simplicity
	FeeTotalLoki                 int64     `json:"fee"`    // Mapped to 'fee' for frontend simplicity
}

type Lsps5Webhook struct {
	ClientNodeId string
	AppName      string
	Url          string
	Transport    string // "push" or "nostr"
	CreatedAt    time.Time
}

type OpeningStore interface {
	GetFeeParamsSettings(token string) ([]*OpeningFeeParamsSetting, error)
	SetFeeParams(token string, validity time.Duration, params OpeningFeeParams) error
	RegisterLsps2Buy(reg *Lsps2BuyRegistration) error
	GetLsps2BuyRegistration(scid uint64) (*Lsps2BuyRegistration, error)
	RegisterLsps1Order(order *Lsps1Order) error
	GetLsps1Order(orderId string) (*Lsps1Order, error)
	GetLsps1OrderByPaymentInvoice(paymentInvoice string) (*Lsps1Order, error)
	GetPaidLsps1Order(clientNodeId string) (*Lsps1Order, error)
	GetPaidLsps1Orders(clientNodeId string) ([]*Lsps1Order, error)
	GetAllPaidLsps1Orders() ([]*Lsps1Order, error)
	ListLsps1Orders(limit, offset int) ([]*Lsps1Order, error)
	UpdateLsps1OrderState(orderId string, newState string, newPaymentState string) error
	UpdateLsps1OrderFunding(orderId string, fundingTxid string, fundingOutpoint int) error
	RegisterLsps5Webhook(webhook *Lsps5Webhook) error
	GetLsps5Webhooks(clientNodeId string) ([]*Lsps5Webhook, error)
	RemoveLsps5Webhook(clientNodeId, appName string) error
	CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error)
	CountLsps5Webhooks(clientNodeId string) (int, error)
	GetReservedLiquidity() (uint64, error)
	ExpireStaleOrders() (int64, error)
}
