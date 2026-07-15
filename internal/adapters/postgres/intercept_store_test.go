package postgres

import (
	"context"
	"testing"

	"github.com/flowgate-lsp/flspd/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var openingFeeParamsFixture = common.OpeningFeeParams{
	MinFeeMloki:  1000,
	Proportional: 4000,
	ValidUntil:   "2099-01-01T00:00:00.000Z",
}

// TestPaymentInfo_NullOpeningFeeParams is the regression test for F-2: a
// payments row with a NULL opening_fee_params (a plain forward, not a
// channel-open payment — the column is nullable) used to panic PaymentInfo
// with a nil-pointer dereference, because extParams was only assigned
// inside the `if p != nil` branch but dereferenced unconditionally on the
// final return.
func TestPaymentInfo_NullOpeningFeeParams(t *testing.T) {
	store := NewPostgresInterceptStore(testPool)

	paymentHash := []byte("f2-test-null-params-hash-32bytes")
	require.Len(t, paymentHash, 32, "payment_hash is a bytea column keyed by hash; keep it a realistic fixed size")

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO public.payments
			(payment_hash, payment_secret, destination, incoming_amount_mloki, outgoing_amount_mloki, opening_fee_params)
		 VALUES ($1, $2, $3, $4, $5, NULL)
		 ON CONFLICT (payment_hash) DO NOTHING`,
		paymentHash, []byte("secret"), []byte("dest"), int64(1000), int64(990),
	)
	require.NoError(t, err)

	token, orderId, params, gotHash, gotSecret, gotDest, incoming, outgoing, cp, tag, err := store.PaymentInfo(paymentHash)

	assert.NoError(t, err, "PaymentInfo must not error (and, per F-2, must not panic) on a row with NULL opening_fee_params")
	assert.Equal(t, "", token)
	assert.Equal(t, "", orderId)
	assert.Nil(t, params)
	assert.Equal(t, paymentHash, gotHash)
	assert.Equal(t, []byte("secret"), gotSecret)
	assert.Equal(t, []byte("dest"), gotDest)
	assert.Equal(t, int64(1000), incoming)
	assert.Equal(t, int64(990), outgoing)
	assert.Nil(t, cp)
	assert.Nil(t, tag)
}

// TestPaymentInfo_WithOpeningFeeParams is the counterpart happy path: a row
// that DOES have opening_fee_params should still decode correctly, so the
// F-2 fix's early-return-on-nil branch doesn't regress the normal case.
func TestPaymentInfo_WithOpeningFeeParams(t *testing.T) {
	store := NewPostgresInterceptStore(testPool)

	paymentHash := []byte("f2-test-with-params-hash-32bytes")
	require.Len(t, paymentHash, 32)

	err := store.RegisterPayment("token-abc", "order-123", &openingFeeParamsFixture, []byte("dest"), paymentHash, []byte("secret"), 2000, 1990, "")
	require.NoError(t, err)

	token, orderId, params, _, _, _, incoming, outgoing, _, _, err := store.PaymentInfo(paymentHash)

	assert.NoError(t, err)
	assert.Equal(t, "token-abc", token)
	assert.Equal(t, "order-123", orderId)
	require.NotNil(t, params)
	assert.Equal(t, uint64(1000), params.MinFeeMloki)
	assert.Equal(t, int64(2000), incoming)
	assert.Equal(t, int64(1990), outgoing)
}

// TestInterceptStore_GetLsps2Order_RoundTrip covers GetLsps2Order, which
// previously had zero test coverage. It reads the same lsps2.buy_registrations
// table PostgresOpeningStore.RegisterLsps2Buy writes to, via the identical
// int64(scid)/uint64(scidInt) cast — including a SCID with bit 63 set,
// where int64(scid) is negative, to prove the cast round-trips correctly
// from this read path too (see TestOpeningStore_Lsps2BuyRegistration_HighBitScid
// for why that matters).
func TestInterceptStore_GetLsps2Order_RoundTrip(t *testing.T) {
	openingStore := NewPostgresOpeningStore(testPool)
	interceptStore := NewPostgresInterceptStore(testPool)

	const highBlockHeight = 9_500_000 // > 2^23; makes int64(scid) negative
	scid := uint64(highBlockHeight)<<40 | uint64(2)<<16 | uint64(0)
	require.Negative(t, int64(scid), "sanity: fixture must set bit 63")

	paymentSize := uint64(25_000_000)
	reg := &common.Lsps2BuyRegistration{
		Id:                         newOrderId(t),
		LspId:                      "lsp-pubkey-intercept",
		PeerId:                     "peer-pubkey-intercept",
		Scid:                       scid,
		Mode:                       1,
		PaymentSizeMloki:           &paymentSize,
		ParamsMinFeeMloki:          2000,
		ParamsProportional:         5000,
		ParamsValidUntil:           "2099-01-01T00:00:00.000Z",
		ParamsMinLifetime:          43200,
		ParamsMaxClientToSelfDelay: 4320,
		ParamsPromise:              "cafef00d",
		Token:                      "order-token-xyz",
	}
	require.NoError(t, openingStore.RegisterLsps2Buy(reg))

	got, err := interceptStore.GetLsps2Order(scid)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, scid, got.Scid, "GetLsps2Order must round-trip a bit-63-set SCID exactly")
	assert.Equal(t, reg.LspId, got.LspId)
	assert.Equal(t, reg.PeerId, got.PeerId)
	assert.Equal(t, reg.Token, got.Token)
	assert.Equal(t, reg.ParamsPromise, got.ParamsPromise)
}

// TestInterceptStore_GetLsps2Order_NotFound covers the no-such-SCID branch,
// which must return (nil, nil) rather than an error (callers use a nil
// result to mean "not a JIT-channel payment").
func TestInterceptStore_GetLsps2Order_NotFound(t *testing.T) {
	interceptStore := NewPostgresInterceptStore(testPool)

	got, err := interceptStore.GetLsps2Order(uint64(1)<<62 | 0xdeadbeef)
	assert.NoError(t, err)
	assert.Nil(t, got)
}
