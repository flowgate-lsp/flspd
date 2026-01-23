package itest

import (
	"log"
	"time"

	"github.com/flowgate-lsp/flspd/itest/lntest"
	rpc "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

func testZeroReserve(p *testParams) {
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
	log.Printf("Alice paying")
	route := constructRoute(p.lsp.LightningNode(), p.LspClient().Node(), channelId, lntest.NewShortChanIDFromString("1x0x0"), outerAmountMloki)
	alice.PayViaRoute(outerAmountMloki, outerInvoice.paymentHash, outerInvoice.paymentSecret, route)

	// Make sure balance is correct
	chans := p.LspClient().Node().GetChannels()
	assert.Equal(p.t, 1, len(chans))

	c := chans[0]
	assert.Equal(p.t, c.RemoteReserveMloki, c.CapacityMloki/100)
	log.Printf("local reserve: %d, remote reserve: %d", c.LocalReserveMloki, c.RemoteReserveMloki)
	assert.Zero(p.t, c.LocalReserveMloki)

	lspInvoice := p.lsp.LightningNode().CreateBolt11Invoice(&lntest.CreateInvoiceOptions{
		AmountMloki: innerAmountMloki - 1,
	})
	<-time.After(time.Second * 1)
	p.LspClient().Node().Pay(lspInvoice.Bolt11)
}
