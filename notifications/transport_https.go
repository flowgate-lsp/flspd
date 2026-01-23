package notifications

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/rs/zerolog/log"
)

// HTTPSTransport implements NotificationTransport for HTTPS webhook delivery.
// It sends bLIP-55 compliant JSON-RPC 2.0 notifications with signed headers.
type HTTPSTransport struct {
	nodePrivKey interface{} // *btcec.PrivateKey
}

// NewHTTPSTransport creates a new HTTPS webhook transport
func NewHTTPSTransport(nodePrivKey interface{}) *HTTPSTransport {
	return &HTTPSTransport{
		nodePrivKey: nodePrivKey,
	}
}

// Type returns TransportTypePush
func (t *HTTPSTransport) Type() TransportType {
	return TransportTypePush
}

// Send delivers a notification via HTTPS POST to the webhook URL.
// It creates a JSON-RPC 2.0 notification, signs it per bLIP-55, and includes
// the required x-lsps5-timestamp and x-lsps5-signature headers.
func (t *HTTPSTransport) Send(ctx context.Context, webhookURL string, method string, params interface{}) error {
	// Type assert nodePrivKey
	privKey, ok := t.nodePrivKey.(*btcec.PrivateKey)
	if !ok {
		log.Error().
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Msgf("HTTPSTransport: invalid private key type: %T", t.nodePrivKey)
		return ErrInvalidPrivateKey
	}

	// Create JSON-RPC 2.0 notification body
	body, err := lightning.CreateLsps5Notification(method, params)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("method", method).
			Msg("HTTPSTransport: Failed to create LSPS5 notification body")
		return err
	}

	// Generate ISO8601 timestamp per bLIP-55: YYYY-MM-DDThh:mm:ss.uuuZ
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Sign the message per bLIP-55 specification
	signature, err := lightning.SignLsps5Message(privKey, timestamp, body)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("method", method).
			Msg("HTTPSTransport: Failed to sign LSPS5 notification")
		return err
	}

	// Create HTTP POST request with required headers
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("url", webhookURL).
			Msg("HTTPSTransport: Failed to create HTTP request")
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-lsps5-timestamp", timestamp)
	req.Header.Set("x-lsps5-signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("url", webhookURL).
			Str("method", method).
			Msg("HTTPSTransport: Webhook delivery failed (network error)")
		return err
	}
	defer resp.Body.Close()

	// Per bLIP-55: only 200 OK is considered success
	if resp.StatusCode != http.StatusOK {
		buf := make([]byte, 1000)
		bytesRead, _ := io.ReadFull(resp.Body, buf)
		respBody := buf[:bytesRead]
		log.Error().
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("url", webhookURL).
			Str("method", method).
			Str("status", resp.Status).
			Str("body", string(respBody)).
			Msg("HTTPSTransport: Webhook returned non-200 status")
		return ErrWebhookFailed
	}

	log.Debug().
		Str("component", string(common.LogComponentNotifications)).
		Str("action", string(common.LogActionSuccess)).
		Str("url", webhookURL).
		Str("method", method).
		Msg("HTTPSTransport: Notification delivered successfully")

	return nil
}
