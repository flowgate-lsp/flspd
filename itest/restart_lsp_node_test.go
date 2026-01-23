package itest

import (
	"log"
	"time"

	"github.com/flowgate-lsp/flspd/itest/lntest"
	rpc "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

func testRestartLspNode(p *testParams) {
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	p.lsp.LightningNode().Fund(10000000)

	log.Print("Opening channel between Alice and the lsp")
	channel := alice.OpenChannel(p.lsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountLoki: publicChanAmount,
	})
	alice.WaitForChannelReady(channel)

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

	log.Printf("stopping lsp lightning node")
	p.lsp.LightningNode().Stop()
	log.Printf("waiting %v to allow lsp lightning node to stop completely", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)
	log.Printf("starting lsp lightning node again")
	p.lsp.LightningNode().Start()

	// TODO: Fix race waiting for htlc interceptor.
	log.Printf("Waiting %v to allow htlc interceptor to activate.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)

	log.Printf("Connect Bob to LSP again")
	p.LspClient().Node().ConnectPeer(p.lsp.LightningNode())
	alice.WaitForChannelReady(channel)

	log.Printf("Alice paying")
	payResp := alice.Pay(outerInvoice.bolt11)
	bobInvoice := p.LspClient().Node().GetInvoice(payResp.PaymentHash)

	assert.Equal(p.t, payResp.PaymentPreimage, bobInvoice.PaymentPreimage)
	assert.Equal(p.t, innerAmountMloki, bobInvoice.AmountReceivedMloki)

	// Make sure capacity is correct
	chans := p.LspClient().Node().GetChannels()
	assert.Equal(p.t, 1, len(chans))
	c := chans[0]
	AssertChannelCapacity(p.t, outerAmountMloki, c.CapacityMloki)
}
