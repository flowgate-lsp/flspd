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

func testNostrNotification(p *testParams) {
	// 1. Setup Nostr Keys
	lspNsec := nostr.GeneratePrivateKey()
	// lspNpub, _ := nostr.GetPublicKey(lspNsec)

	clientNsec := nostr.GeneratePrivateKey()
	clientNpub, _ := nostr.GetPublicKey(clientNsec)

	// 2. Start Mock Relay
	relay := NewMockRelay("127.0.0.1:0")
	err := relay.Start(p.h.Ctx)
	assert.NoError(p.t, err)
	defer relay.Stop()

	log.Printf("Mock Nostr Relay started at %s", relay.Addr())

	// 3. Configure LSP with Nostr
	p.lsp.Stop()

	// Re-initialize LSP with Nostr env
	envExt := []string{
		fmt.Sprintf("LSP_NOSTR_PRIVATE_KEY=%s", lspNsec),
		fmt.Sprintf("LSP_NOSTR_RELAYS=%s", relay.Addr()),
	}

	// Create a new LSP node for this test
	customLsp := NewLndLspdNode(p.h, p.m, p.mem, "LSP-Nostr", nil, envExt...)
	// assert.NoError(p.t, err) // NewLndLspdNode doesn't return error
	customLsp.Start()
	defer customLsp.Stop()

	// 4. Setup Alice and Bob
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	customLsp.LightningNode().Fund(10000000)

	channel := alice.OpenChannel(customLsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountLoki: publicChanAmount,
	})
	channelId := alice.WaitForChannelReady(channel)

	// 5. Connect Client and set up HTLC acceptor
	p.LspClient().Node().ConnectPeer(customLsp.LightningNode())

	// Create Invoices
	outerAmountMloki := uint64(2100000)
	innerAmountMloki := calculateInnerAmountMloki(customLsp, outerAmountMloki, nil)
	description := "Please pay me via Nostr"
	// Note: We need to use customLsp here, not p.lsp
	// But GenerateInvoices takes LspClient which needs a testParams
	// We can manually call generateInvoices or hack p.lsp to point to customLsp
	// p.lsp is LspNode interface.

	// Let's manually generate invoices
	preimage, err := GenerateRandomBytes(32)
	assert.NoError(p.t, err)

	innerInvoice := p.LspClient().Node().CreateBolt11Invoice(&lntest.CreateInvoiceOptions{
		AmountMloki: innerAmountMloki,
		Description: &description,
		Preimage:    &preimage,
	})

	// AddHopHint needs LspNode, pass customLsp
	scid := lntest.ShortChannelID{BlockHeight: 1, TxIndex: 0, OutputIndex: 0}
	_ = AddHopHint(p.LspClient(), innerInvoice.Bolt11, customLsp, scid, &outerAmountMloki, lspCltvDelta)
	// Outer invoice payment hash is same as inner? No, AddHopHint modifies invoice but hash stays same?
	// AddHopHint decodes and re-encodes, hash depends on content.
	// But in LN, hash is hash of preimage. Preimage is same. So hash is same.
	paymentHash := innerInvoice.PaymentHash
	paymentSecret := innerInvoice.PaymentSecret

	// Register Payment
	log.Printf("Registering payment with lsp")
	RegisterPayment(customLsp, &rpc.PaymentInformation{
		PaymentHash:         paymentHash,
		PaymentSecret:       paymentSecret,
		Destination:         p.LspClient().Node().NodeId(),
		IncomingAmountMloki: int64(outerAmountMloki),
		OutgoingAmountMloki: int64(innerAmountMloki),
	}, false)

	// 6. Subscribe to Nostr Notifications
	// Register the Client's Npub as the webhook URL with transport "nostr"
	// New format: nostr+lsps5://<pubkey>?relay=<relay_url>
	relayUrl := "ws://" + relay.Addr()
	webhookUrl := fmt.Sprintf("nostr+lsps5://%s?relay=%s", clientNpub, relayUrl)
	err = SubscribeNotifications(customLsp, p.LspClient(), webhookUrl, "nostr", false)
	assert.NoError(p.t, err)

	// 7. Make Payment
	p.LspClient().SetHtlcAcceptor(innerAmountMloki)
	log.Printf("Waiting %v to allow htlc interceptor to activate.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)

	log.Printf("Alice paying")
	route := constructRoute(customLsp.LightningNode(), p.LspClient().Node(), channelId, lntest.NewShortChanIDFromString("1x0x0"), outerAmountMloki)
	_, err = alice.PayViaRoute(outerAmountMloki, paymentHash, paymentSecret, route)
	assert.Nil(p.t, err)

	// 8. Verify Nostr Event
	assert.Eventually(p.t, func() bool {
		events := relay.Events()
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
			lspNpub, _ := nostr.GetPublicKey(lspNsec) // Ideally get it from customLsp or config

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
	}, 10*time.Second, 500*time.Millisecond, "Expected LSPS5 notification not found on relay")
}
