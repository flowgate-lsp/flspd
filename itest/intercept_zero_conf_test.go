package itest

import (
	"log"
	"time"

	"github.com/flowgate-lsp/flspd/itest/lntest"
	rpc "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

var htlcInterceptorDelay = time.Second * 7

func testOpenZeroConfChannelOnReceive(p *testParams) {
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

	// TODO: Fix race waiting for htlc interceptor.
	log.Printf("Waiting %v to allow htlc interceptor to activate.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)
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

func testOpenZeroConfSingleHtlc(p *testParams) {
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
	payResp, err := alice.PayViaRoute(outerAmountMloki, outerInvoice.paymentHash, outerInvoice.paymentSecret, route)
	lntest.CheckError(p.t, err)
	bobInvoice := p.LspClient().Node().GetInvoice(payResp.PaymentHash)

	assert.Equal(p.t, payResp.PaymentPreimage, bobInvoice.PaymentPreimage)
	assert.Equal(p.t, innerAmountMloki, bobInvoice.AmountReceivedMloki)

	// Make sure capacity is correct
	chans := p.LspClient().Node().GetChannels()
	assert.Equal(p.t, 1, len(chans))
	c := chans[0]
	AssertChannelCapacity(p.t, outerAmountMloki, c.CapacityMloki)
}

func constructRoute(
	lsp lntest.LightningNode,
	bob lntest.LightningNode,
	aliceLspChannel lntest.ShortChannelID,
	lspBobChannel lntest.ShortChannelID,
	amountMloki uint64,
) *lntest.Route {
	return &lntest.Route{
		Hops: []*lntest.Hop{
			{
				Id:          lsp.NodeId(),
				Channel:     aliceLspChannel,
				AmountMloki: amountMloki + uint64(lspBaseFeeMloki) + (amountMloki * uint64(lspFeeRatePpm) / 1000000),
				Delay:       144 + lspCltvDelta,
			},
			{
				Id:          bob.NodeId(),
				Channel:     lspBobChannel,
				AmountMloki: amountMloki,
				Delay:       144,
			},
		},
	}
}
