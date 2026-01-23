package itest

import (
	"log"
	"time"

	"github.com/flowgate-lsp/flspd/itest/lntest"
	rpc "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

func testDynamicFeeFlow(p *testParams) {
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	p.lsp.LightningNode().Fund(10000000)

	log.Print("Opening channel between Alice and the lsp")
	channel := alice.OpenChannel(p.lsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountLoki: publicChanAmount,
	})
	channelId := alice.WaitForChannelReady(channel)

	log.Printf("Getting channel information")
	SetFeeParams(p.lsp, []*FeeParamSetting{
		{
			Validity:     time.Second * 3600,
			MinMloki:     3000000,
			Proportional: 1000,
		},
	},
	)
	info := ChannelInformation(p.lsp)
	assert.Len(p.t, info.OpeningFeeParamsMenu, 1)
	params := info.OpeningFeeParamsMenu[0]
	assert.Equal(p.t, uint64(3000000), params.MinMloki)

	log.Printf("opening_fee_params: %+v", params)
	log.Printf("Adding bob's invoices")
	outerAmountMloki := uint64(4200000)
	innerAmountMloki := calculateInnerAmountMloki(p.lsp, outerAmountMloki, params)
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

	log.Printf("Testing some bad registrations")
	err := RegisterPayment(p.lsp, &rpc.PaymentInformation{
		PaymentHash:         innerInvoice.paymentHash,
		PaymentSecret:       innerInvoice.paymentSecret,
		Destination:         p.LspClient().Node().NodeId(),
		IncomingAmountMloki: int64(outerAmountMloki),
		OutgoingAmountMloki: int64(innerAmountMloki),
		OpeningFeeParams: &rpc.OpeningFeeParams{
			// modify minmloki
			MinMloki:             params.MinMloki + 1,
			Proportional:         params.Proportional,
			ValidUntil:           params.ValidUntil,
			MaxIdleTime:          params.MaxIdleTime,
			MaxClientToSelfDelay: params.MaxClientToSelfDelay,
			Promise:              params.Promise,
		},
	}, true)
	assert.Contains(p.t, err.Error(), "invalid opening_fee_params")

	err = RegisterPayment(p.lsp, &rpc.PaymentInformation{
		PaymentHash:         innerInvoice.paymentHash,
		PaymentSecret:       innerInvoice.paymentSecret,
		Destination:         p.LspClient().Node().NodeId(),
		IncomingAmountMloki: int64(outerAmountMloki),
		OutgoingAmountMloki: int64(innerAmountMloki),
		OpeningFeeParams: &rpc.OpeningFeeParams{
			MinMloki:             params.MinMloki,
			Proportional:         params.Proportional,
			ValidUntil:           params.ValidUntil,
			MaxIdleTime:          params.MaxIdleTime,
			MaxClientToSelfDelay: params.MaxClientToSelfDelay,
			// Modify promise
			Promise: params.Promise + "aa",
		},
	}, true)
	assert.Contains(p.t, err.Error(), "invalid opening_fee_params")

	err = RegisterPayment(p.lsp, &rpc.PaymentInformation{
		PaymentHash:   innerInvoice.paymentHash,
		PaymentSecret: innerInvoice.paymentSecret,
		Destination:   p.LspClient().Node().NodeId(),
		// Fee too low
		IncomingAmountMloki: int64(2999999),
		OutgoingAmountMloki: int64(0),
		OpeningFeeParams:    params,
	}, true)
	assert.Contains(p.t, err.Error(), "not enough fees")

	err = RegisterPayment(p.lsp, &rpc.PaymentInformation{
		PaymentHash:         innerInvoice.paymentHash,
		PaymentSecret:       innerInvoice.paymentSecret,
		Destination:         p.LspClient().Node().NodeId(),
		IncomingAmountMloki: int64(outerAmountMloki),
		OutgoingAmountMloki: int64(innerAmountMloki + 1),
		OpeningFeeParams:    params,
	}, true)
	assert.Contains(p.t, err.Error(), "not enough fees")

	// Now register the payment for real
	log.Printf("Registering payment with lsp")
	RegisterPayment(p.lsp, &rpc.PaymentInformation{
		PaymentHash:         innerInvoice.paymentHash,
		PaymentSecret:       innerInvoice.paymentSecret,
		Destination:         p.LspClient().Node().NodeId(),
		IncomingAmountMloki: int64(outerAmountMloki),
		OutgoingAmountMloki: int64(innerAmountMloki),
		OpeningFeeParams:    info.OpeningFeeParamsMenu[0],
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
