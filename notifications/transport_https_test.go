package notifications

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPrivKey(t *testing.T) *btcec.PrivateKey {
	t.Helper()
	priv, _ := btcec.PrivKeyFromBytes([]byte("0123456789abcdef0123456789abcde"))
	return priv
}

func TestHTTPSTransport_Send_SignsAndPosts(t *testing.T) {
	priv := testPrivKey(t)

	var gotBody []byte
	var gotTimestamp, gotSignature string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotTimestamp = r.Header.Get("x-lsps5-timestamp")
		gotSignature = r.Header.Get("x-lsps5-signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewHTTPSTransport(priv)
	err := transport.Send(context.Background(), server.URL, "lsps5.payment_incoming", map[string]string{"payment_hash": "abc"})
	require.NoError(t, err)

	assert.Contains(t, string(gotBody), "lsps5.payment_incoming")
	assert.NotEmpty(t, gotTimestamp)
	assert.NotEmpty(t, gotSignature)

	// The signature must actually verify against the timestamp+body it was
	// sent with, not just be present.
	pubkey, err := lightning.VerifyMessage(
		[]byte("LSPS5: DO NOT SIGN THIS MESSAGE MANUALLY: LSP: At "+gotTimestamp+" I notify "+string(gotBody)),
		gotSignature,
	)
	require.NoError(t, err)
	assert.True(t, priv.PubKey().IsEqual(pubkey))
}

func TestHTTPSTransport_Send_NonOKStatusIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	transport := NewHTTPSTransport(testPrivKey(t))
	err := transport.Send(context.Background(), server.URL, "lsps5.payment_incoming", map[string]string{})
	assert.ErrorIs(t, err, ErrWebhookFailed)
}

func TestHTTPSTransport_Send_InvalidPrivateKeyType(t *testing.T) {
	transport := NewHTTPSTransport("not a private key")
	err := transport.Send(context.Background(), "http://example.invalid", "lsps5.payment_incoming", map[string]string{})
	assert.ErrorIs(t, err, ErrInvalidPrivateKey)
}

func TestHTTPSTransport_Type(t *testing.T) {
	transport := NewHTTPSTransport(testPrivKey(t))
	assert.Equal(t, TransportTypePush, transport.Type())
}
