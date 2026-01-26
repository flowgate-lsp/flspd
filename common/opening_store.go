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
	OrderId                      string
	PeerId                       string
	LspBalanceLoki               int64
	ClientBalanceLoki            int64
	FundingConfirmsWithinBlocks  int
	RequiredChannelConfirmations int
	ChannelExpiryBlocks          int
	Token                        string
	AnnounceChannel              bool
	CreatedAt                    time.Time
	OrderState                   string
	PaymentState                 string
	PaymentInvoice               string
	FundingTxid                  *string
	FundingOutpoint              *int
	RefundOnchainAddress         string
	OrderTotalLoki               int64
	FeeTotalLoki                 int64
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
