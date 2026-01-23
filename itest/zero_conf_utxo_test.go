package itest

import (
	"log"
	"time"

	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/itest/lntest"
	rpc "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

func testOpenZeroConfUtxo(p *testParams) {
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)

	minConfs := uint32(0)
	lsp := p.lspFunc(p.h, p.m, p.mem, &config.NodeConfig{MinConfs: &minConfs})
	lsp.Start()

	log.Print("Opening channel between Alice and the lsp")
	channel := alice.OpenChannel(lsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountLoki: publicChanAmount,
	})
	channelId := alice.WaitForChannelReady(channel)

	tempaddr := lsp.LightningNode().GetNewAddress()
	p.m.SendToAddress(tempaddr, 210000)
	reserveaddr := lsp.LightningNode().GetNewAddress()
	p.m.SendToAddress(reserveaddr, 50000)
	p.m.MineBlocks(6)
	lsp.LightningNode().WaitForSync()

	initialHeight := p.m.GetBlockHeight()
	addr := lsp.LightningNode().GetNewAddress()
	lsp.LightningNode().SendToAddress(addr, 200000)

	log.Printf("Adding bob's invoices")
	outerAmountMloki := uint64(2100000)
	innerAmountMloki := calculateInnerAmountMloki(lsp, outerAmountMloki, nil)
	description := "Please pay me"
	innerInvoice, outerInvoice := GenerateInvoices(p.LspClient(),
		generateInvoicesRequest{
			innerAmountMloki: innerAmountMloki,
			outerAmountMloki: outerAmountMloki,
			description:      description,
			lsp:              lsp,
		})
	p.LspClient().SetHtlcAcceptor(innerAmountMloki)

	log.Print("Connecting bob to flspd")
	p.LspClient().Node().ConnectPeer(lsp.LightningNode())

	log.Printf("Registering payment with lsp")
	RegisterPayment(lsp, &rpc.PaymentInformation{
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
	route := constructRoute(lsp.LightningNode(), p.LspClient().Node(), channelId, lntest.NewShortChanIDFromString("1x0x0"), outerAmountMloki)
	payResp, err := alice.PayViaRoute(outerAmountMloki, outerInvoice.paymentHash, outerInvoice.paymentSecret, route)
	lntest.CheckError(p.t, err)
	bobInvoice := p.LspClient().Node().GetInvoice(payResp.PaymentHash)

	assert.Equal(p.t, payResp.PaymentPreimage, bobInvoice.PaymentPreimage)
	assert.Equal(p.t, innerAmountMloki, bobInvoice.AmountReceivedMloki)

	// Make sure there's not accidently a block mined in between
	finalHeight := p.m.GetBlockHeight()
	assert.Equal(p.t, initialHeight, finalHeight)
}
