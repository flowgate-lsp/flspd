package itest

import (
	"crypto/sha256"
	"log"
	"time"

	"github.com/flowgate-lsp/flspd/itest/lntest"
	rpc "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

func testProbing(p *testParams) {
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	p.lsp.LightningNode().Fund(10000000)

	log.Print("Opening channel between Alice and the lsp")
	channel := alice.OpenChannel(p.lsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountLoki: publicChanAmount,
	})
	channelId := alice.WaitForChannelReady(channel)

	log.Printf("Adding bob's invoices")
	outerAmountMloki := uint64(2100000)
	innerAmountMloki := calculateInnerAmountMloki(p.lsp, outerAmountMloki, nil)
	description := "Please pay me"
	innerInvoice, outerInvoice := GenerateInvoices(p.LspClient(),
		generateInvoicesRequest{
			innerAmountMloki: innerAmountMloki,
			outerAmountMloki: outerAmountMloki,
			description:      description,
			lsp:              p.lsp,
		})
	p.LspClient().SetHtlcAcceptor(innerAmountMloki)

	log.Print("Connecting bob to flspd")
	p.LspClient().Node().ConnectPeer(p.lsp.LightningNode())

	log.Printf("Registering payment with lsp")
	RegisterPayment(p.lsp, &rpc.PaymentInformation{
		PaymentHash:         innerInvoice.paymentHash,
		PaymentSecret:       innerInvoice.paymentSecret,
		Destination:         p.LspClient().Node().NodeId(),
		IncomingAmountMloki: int64(outerAmountMloki),
		OutgoingAmountMloki: int64(innerAmountMloki),
	}, false)

	// TODO: Fix race waiting for htlc interceptor.
	log.Printf("Waiting %v to allow htlc interceptor to activate.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)

	h := sha256.New()
	_, _ = h.Write([]byte("probing-01:"))
	_, _ = h.Write(outerInvoice.paymentHash)
	fakePaymentHash := h.Sum(nil)

	log.Printf("Alice paying with fake payment hash with Bob online %x", fakePaymentHash)
	route := constructRoute(p.lsp.LightningNode(), p.LspClient().Node(), channelId, lntest.NewShortChanIDFromString("1x0x0"), outerAmountMloki)
	_, err := alice.PayViaRoute(outerAmountMloki, fakePaymentHash, outerInvoice.paymentSecret, route)

	// Expect incorrect or unknown payment details if the peer is online
	assert.Contains(p.t, err.Error(), "WIRE_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS")

	// Kill the mobile client
	log.Printf("Stopping client")
	p.LspClient().Stop()

	log.Printf("Alice paying with fake payment hash with Bob offline %x", fakePaymentHash)
	_, err = alice.PayViaRoute(outerAmountMloki, fakePaymentHash, outerInvoice.paymentSecret, route)

	// Expect unknown next peer if the peer is offline
	assert.Contains(p.t, err.Error(), "WIRE_UNKNOWN_NEXT_PEER")
}
