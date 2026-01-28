package itest

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/flowgate-lsp/flspd/itest/lntest"
	"github.com/flowgate-lsp/flspd/rpc"
	lsps5 "github.com/flowgate-lsp/nostr-lsps5"
	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/assert"
)

func testNostrNotificationRetry(p *testParams) {
	// 1. Setup Nostr Keys
	lspNsec := nostr.GeneratePrivateKey()
	clientNsec := nostr.GeneratePrivateKey()
	clientNpub, _ := nostr.GetPublicKey(clientNsec)

	// 2. Start Mock Relay
	relay := NewMockRelay("127.0.0.1:0")
	err := relay.Start(p.h.Ctx)
	assert.NoError(p.t, err)
	// We will manage stopping manually in the test

	log.Printf("Mock Nostr Relay started at %s", relay.Addr())

	// 3. Configure LSP with Nostr
	p.lsp.Stop()

	// Re-initialize LSP with Nostr env
	envExt := []string{
		fmt.Sprintf("LSP_NOSTR_PRIVATE_KEY=%s", lspNsec),
		fmt.Sprintf("LSP_NOSTR_RELAYS=%s", relay.Addr()),
	}

	// Create a new LSP node for this test
	customLsp := NewLndLspdNode(p.h, p.m, p.mem, "LSP-Nostr-Retry", nil, envExt...)
	customLsp.Start()
	defer customLsp.Stop()

	// 4. Setup Alice and Channel
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	customLsp.LightningNode().Fund(10000000)

	channel := alice.OpenChannel(customLsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountLoki: publicChanAmount,
	})
	channelId := alice.WaitForChannelReady(channel)

	// 5. Connect Client
	p.LspClient().Node().ConnectPeer(customLsp.LightningNode())

	// 6. Register Payment
	outerAmountMloki := uint64(2100000)
	innerAmountMloki := calculateInnerAmountMloki(customLsp, outerAmountMloki, nil)
	description := "Retry Test Payment"

	preimage, err := GenerateRandomBytes(32)
	assert.NoError(p.t, err)

	innerInvoice := p.LspClient().Node().CreateBolt11Invoice(&lntest.CreateInvoiceOptions{
		AmountMloki: innerAmountMloki,
		Description: &description,
		Preimage:    &preimage,
	})
	paymentHash := innerInvoice.PaymentHash
	paymentSecret := innerInvoice.PaymentSecret

	RegisterPayment(customLsp, &rpc.PaymentInformation{
		PaymentHash:         paymentHash,
		PaymentSecret:       paymentSecret,
		Destination:         p.LspClient().Node().NodeId(),
		IncomingAmountMloki: int64(outerAmountMloki),
		OutgoingAmountMloki: int64(innerAmountMloki),
	}, false)

	// 7. Subscribe to Nostr Notifications
	// Note: We use the relay URL that is currently running
	relayUrl := relay.Addr()
	webhookUrl := fmt.Sprintf("nostr+lsps5://%s?relay=%s", clientNpub, relayUrl)
	err = SubscribeNotifications(customLsp, p.LspClient(), webhookUrl, "nostr", false)
	assert.NoError(p.t, err)

	// 8. CRITICAL STEP: STOP RELAY
	log.Printf("Stopping relay to force failure and enqueueing...")
	relay.Stop()

	// 9. Make Payment (Should trigger notification -> fail -> enqueue)
	p.LspClient().SetHtlcAcceptor(innerAmountMloki)
	log.Printf("Waiting %v for intercepts.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)

	log.Printf("Alice paying (expecting notification failure)")
	route := constructRoute(customLsp.LightningNode(), p.LspClient().Node(), channelId, lntest.NewShortChanIDFromString("1x0x0"), outerAmountMloki)
	_, err = alice.PayViaRoute(outerAmountMloki, paymentHash, paymentSecret, route)
	assert.Nil(p.t, err)

	// 10. Wait a bit to ensure it failed and is in DB (backoff 10s)
	log.Printf("Waiting for potential first retry failure...")
	time.Sleep(5 * time.Second)

	// 11. RESTART RELAY
	// NewMockRelay creates a new listener, likely different port if 0 is used.
	// We need to restart on the SAME port because the LSP config and Webhook URL point to the old address.
	// Our MockRelay implementation in nostr_relay.go listens on new random port if 0 is passed.
	// But `relay.Addr()` gave us the actual address. We should try to listen on that.
	// However, `MockRelay` struct doesn't support restarting specifically on same port easily unless we modify it or exact same address string.
	// `relay.addr` is updated in Start.

	// Let's create a NEW mock relay instance but try to reuse the address.
	// The `relay.addr` is "127.0.0.1:xxxxx".
	// Stopping the old one frees the port.
	log.Printf("Restarting relay at %s", relay.Addr())

	// We need to use the SAME address so the LSP connects to it.
	// The `relay.addr` field holds the listener address.
	// We can try to just call Start again? No, Start creates new server.

	// Let's hack it: Create new MockRelay with the old address.
	// oldAddr := relay.Addr()
	// relay.addr is "127.0.0.1:4567" (handled in Addr() adding ws://)
	// We need the raw host:port.
	// The `MockRelay` struct has `addr` field which seems to be the listener address.

	// Issue: `relay.Addr()` returns `ws://...`. the `addr` field is private.
	// But `NewMockRelay` takes addr string.
	// Let's parse the port.

	// Actually, `relay.Stop()` closes the listener.
	// We can just construct a new one with the same host:port.
	// The only issue is if the socket is in TIME_WAIT, but typically fine in tests.

	// The previous relay.Addr() returns `ws://IP:PORT`.
	// We need `IP:PORT`.
	targetAddr := relay.Addr()[5:] // strip ws://

	relay2 := NewMockRelay(targetAddr)
	err = relay2.Start(p.h.Ctx)
	assert.NoError(p.t, err)
	defer relay2.Stop()

	log.Printf("Relay restarted. Waiting for retry (approx 10s backoff + ticker)...")

	// 12. Verify Nostr Event Eventually Arrives
	assert.Eventually(p.t, func() bool {
		events := relay2.Events()
		for _, ev := range events {
			if ev.Kind != lsps5.KindLSPS5 {
				continue
			}

			// Verify Tags
			hasPTag := false
			for _, tag := range ev.Tags {
				if tag[0] == "p" && tag[1] == clientNpub {
					hasPTag = true
					break
				}
			}
			if !hasPTag {
				continue
			}

			// Decrypt Content
			clientLSPS5 := lsps5.NewLSPS5(clientNsec)
			lspNpub, _ := nostr.GetPublicKey(lspNsec)

			notification, err := clientLSPS5.ParseNotificationEvent(ev, lspNpub)
			if err != nil {
				log.Printf("Failed to parse event: %v", err)
				continue
			}

			if notification.Method == "lsps5.payment_incoming" {
				params := notification.Params.(map[string]interface{})
				if hash, ok := params["payment_hash"].(string); ok {
					if hash == hex.EncodeToString(paymentHash) {
						return true
					}
				}
			}
		}
		return false
	}, 30*time.Second, 1*time.Second, "Expected LSPS5 notification not found on relay after retry")
}
