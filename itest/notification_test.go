package itest

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/flowgate-lsp/flspd/itest/lntest"
	lspd "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

func testOfflineNotificationPaymentRegistered(p *testParams) {
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	p.lsp.LightningNode().Fund(10000000)

	log.Print("Opening channel between Alice and the lsp")
	channel := alice.OpenChannel(p.lsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountSat: publicChanAmount,
	})
	channelId := alice.WaitForChannelReady(channel)

	log.Printf("Adding bob's invoices")
	outerAmountMsat := uint64(2100000)
	innerAmountMsat := calculateInnerAmountMsat(p.lsp, outerAmountMsat, nil)
	description := "Please pay me"
	innerInvoice, outerInvoice := GenerateInvoices(p.LspClient(),
		generateInvoicesRequest{
			innerAmountMsat: innerAmountMsat,
			outerAmountMsat: outerAmountMsat,
			description:     description,
			lsp:             p.lsp,
		})

	log.Print("Connecting bob to lspd")
	p.LspClient().Node().ConnectPeer(p.lsp.LightningNode())

	log.Printf("Registering payment with lsp")
	RegisterPayment(p.lsp, &lspd.PaymentInformation{
		PaymentHash:        innerInvoice.paymentHash,
		PaymentSecret:      innerInvoice.paymentSecret,
		Destination:        p.LspClient().Node().NodeId(),
		IncomingAmountMsat: int64(outerAmountMsat),
		OutgoingAmountMsat: int64(innerAmountMsat),
	}, false)

	// Kill the mobile client
	log.Printf("Stopping client")
	p.LspClient().Stop()

	port, err := lntest.GetPort()
	if err != nil {
		assert.FailNow(p.t, "failed to get port for deliveeery service")
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	delivered := make(chan struct{})

	notify := newNotificationDeliveryService(addr, func(resp http.ResponseWriter, req *http.Request) {
		var body PaymentReceivedPayload
		err = json.NewDecoder(req.Body).Decode(&body)
		assert.NoError(p.t, err)

		ph := hex.EncodeToString(innerInvoice.paymentHash)
		assert.Equal(p.t, ph, body.Data.PaymentHash)
		close(delivered)
	})
	go notify.Start(p.h.Ctx)
	go func() {
		<-delivered
		log.Printf("Starting client again")
		p.LspClient().SetHtlcAcceptor(innerAmountMsat)
		p.LspClient().Start()
		p.LspClient().Node().ConnectPeer(p.lsp.LightningNode())
	}()

	// TODO: Fix race waiting for htlc interceptor.
	log.Printf("Waiting %v to allow htlc interceptor to activate.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)

	url := "http://" + addr + "/api/v1/notify"
	SubscribeNotifications(p.lsp, p.LspClient(), url, false)
	log.Printf("Alice paying")
	route := constructRoute(p.lsp.LightningNode(), p.LspClient().Node(), channelId, lntest.NewShortChanIDFromString("1x0x0"), outerAmountMsat)
	_, err = alice.PayViaRoute(outerAmountMsat, outerInvoice.paymentHash, outerInvoice.paymentSecret, route)
	assert.Nil(p.t, err)
}

func testOfflineNotificationRegularForward(p *testParams) {
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	p.lsp.LightningNode().Fund(10000000)
	p.LspClient().Node().Fund(100000)

	log.Print("Opening channel between Alice and the lsp")
	channelAL := alice.OpenChannel(p.lsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountSat: publicChanAmount,
		IsPublic:  true,
	})

	log.Print("Opening channel between lsp and client")
	channelLB := p.lsp.LightningNode().OpenChannel(p.LspClient().Node(), &lntest.OpenChannelOptions{
		AmountSat: 200000,
		IsPublic:  false,
	})

	log.Print("Waiting for channel between Alice and the lsp to be ready.")
	alice.WaitForChannelReady(channelAL)
	log.Print("Waiting for channel between LSP and Bob to be ready.")
	p.lsp.LightningNode().WaitForChannelReady(channelLB)
	p.LspClient().Node().WaitForChannelReady(channelLB)

	port, err := lntest.GetPort()
	if err != nil {
		assert.FailNow(p.t, "failed to get port for deliveeery service")
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	delivered := make(chan struct{})

	notify := newNotificationDeliveryService(addr, func(resp http.ResponseWriter, req *http.Request) {
		var body PaymentReceivedPayload
		err = json.NewDecoder(req.Body).Decode(&body)
		assert.NoError(p.t, err)

		close(delivered)
	})
	go notify.Start(p.h.Ctx)
	go func() {
		<-delivered
		log.Printf("Notification was delivered. Starting client again")
		p.LspClient().Start()
		// p.LspClient().Node().ConnectPeer(p.lsp.LightningNode())
	}()

	url := "http://" + addr + "/api/v1/notify"
	SubscribeNotifications(p.lsp, p.LspClient(), url, false)

	<-time.After(time.Second * 2)
	log.Printf("Adding bob's invoice")
	amountMsat := uint64(2100000)
	bobInvoice := p.LspClient().Node().CreateBolt11Invoice(&lntest.CreateInvoiceOptions{
		AmountMsat:      amountMsat,
		IncludeHopHints: true,
	})
	log.Printf(bobInvoice.Bolt11)

	invoiceWithHint := bobInvoice.Bolt11
	if !ContainsHopHint(p.t, bobInvoice.Bolt11) {
		chans := p.LspClient().Node().GetChannels()
		assert.Len(p.t, chans, 1)

		var id lntest.ShortChannelID
		if chans[0].RemoteAlias != nil {
			id = *chans[0].RemoteAlias
		} else if chans[0].LocalAlias != nil {
			id = *chans[0].LocalAlias
		} else {
			id = chans[0].ShortChannelID
		}
		invoiceWithHint = AddHopHint(p.LspClient(), bobInvoice.Bolt11, p.Lsp(), id, nil, 144)
	}
	log.Printf("invoice with hint: %v", invoiceWithHint)

	log.Printf("Bob going offline")
	p.LspClient().Stop()

	// TODO: Fix race waiting for htlc interceptor.
	log.Printf("Waiting %v to allow htlc interceptor to activate.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)

	log.Printf("Alice paying")
	payResp := alice.Pay(invoiceWithHint)
	invoiceResult := p.LspClient().Node().GetInvoice(bobInvoice.PaymentHash)

	assert.Equal(p.t, payResp.PaymentPreimage, invoiceResult.PaymentPreimage)
	assert.Equal(p.t, amountMsat, invoiceResult.AmountReceivedMsat)
}

func testOfflineNotificationZeroConfChannel(p *testParams) {
	alice := lntest.NewLndNode(p.h, p.m, "Alice")
	alice.Start()
	alice.Fund(10000000)
	p.lsp.LightningNode().Fund(10000000)

	log.Print("Opening channel between Alice and the lsp")
	channel := alice.OpenChannel(p.lsp.LightningNode(), &lntest.OpenChannelOptions{
		AmountSat: publicChanAmount,
		IsPublic:  true,
	})
	channelId := alice.WaitForChannelReady(channel)

	log.Printf("Adding bob's invoices")
	outerAmountMsat := uint64(2100000)
	innerAmountMsat := calculateInnerAmountMsat(p.lsp, outerAmountMsat, nil)
	description := "Please pay me"
	innerInvoice, outerInvoice := GenerateInvoices(p.LspClient(),
		generateInvoicesRequest{
			innerAmountMsat: innerAmountMsat,
			outerAmountMsat: outerAmountMsat,
			description:     description,
			lsp:             p.lsp,
		})

	log.Print("Connecting bob to lspd")
	p.LspClient().Node().ConnectPeer(p.lsp.LightningNode())
	p.LspClient().SetHtlcAcceptor(innerAmountMsat)

	// TODO: Fix race waiting for htlc interceptor.
	log.Printf("Waiting %v to allow htlc interceptor to activate.", htlcInterceptorDelay)
	<-time.After(htlcInterceptorDelay)

	log.Printf("Registering payment with lsp")
	RegisterPayment(p.lsp, &lspd.PaymentInformation{
		PaymentHash:        innerInvoice.paymentHash,
		PaymentSecret:      innerInvoice.paymentSecret,
		Destination:        p.LspClient().Node().NodeId(),
		IncomingAmountMsat: int64(outerAmountMsat),
		OutgoingAmountMsat: int64(innerAmountMsat),
	}, false)

	expectedheight := p.Miner().GetBlockHeight()
	log.Printf("Alice paying")
	route := constructRoute(p.lsp.LightningNode(), p.LspClient().Node(), channelId, lntest.NewShortChanIDFromString("1x0x0"), outerAmountMsat)
	_, err := alice.PayViaRoute(outerAmountMsat, outerInvoice.paymentHash, outerInvoice.paymentSecret, route)
	assert.Nil(p.t, err)

	<-time.After(time.Second * 2)
	log.Printf("Adding bob's invoice for zero conf payment")
	amountMsat := uint64(2100000)

	bobInvoice := p.LspClient().Node().CreateBolt11Invoice(&lntest.CreateInvoiceOptions{
		AmountMsat:      amountMsat,
		IncludeHopHints: true,
	})

	invoiceWithHint := bobInvoice.Bolt11
	if !ContainsHopHint(p.t, bobInvoice.Bolt11) {
		chans := p.LspClient().Node().GetChannels()
		assert.Len(p.t, chans, 1)

		var id lntest.ShortChannelID
		if chans[0].RemoteAlias != nil {
			id = *chans[0].RemoteAlias
		} else if chans[0].LocalAlias != nil {
			id = *chans[0].LocalAlias
		} else {
			id = chans[0].ShortChannelID
		}
		invoiceWithHint = AddHopHint(p.LspClient(), bobInvoice.Bolt11, p.Lsp(), id, nil, lspCltvDelta)
	}

	log.Printf("Invoice with hint: %s", invoiceWithHint)

	// Kill the mobile client
	log.Printf("Stopping client")
	p.LspClient().Stop()
	p.LspClient().ResetHtlcAcceptor()

	port, err := lntest.GetPort()
	if err != nil {
		assert.FailNow(p.t, "failed to get port for delivery service")
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	delivered := make(chan struct{})

	notify := newNotificationDeliveryService(addr, func(resp http.ResponseWriter, req *http.Request) {
		var body PaymentReceivedPayload
		err = json.NewDecoder(req.Body).Decode(&body)
		assert.NoError(p.t, err)

		close(delivered)
	})
	go notify.Start(p.h.Ctx)
	go func() {
		<-delivered
		log.Printf("Starting client again")
		p.LspClient().Start()
		p.LspClient().Node().ConnectPeer(p.lsp.LightningNode())
	}()

	url := "http://" + addr + "/api/v1/notify"
	SubscribeNotifications(p.lsp, p.LspClient(), url, false)

	log.Printf("Alice paying zero conf invoice")
	payResp := alice.Pay(invoiceWithHint)
	invoiceResult := p.LspClient().Node().GetInvoice(bobInvoice.PaymentHash)

	assert.Equal(p.t, payResp.PaymentPreimage, invoiceResult.PaymentPreimage)
	assert.Equal(p.t, amountMsat, invoiceResult.AmountReceivedMsat)

	// Make sure we haven't accidentally mined blocks in between.
	actualheight := p.Miner().GetBlockHeight()
	assert.Equal(p.t, expectedheight, actualheight)
}

func testNotificationsUnsubscribe(p *testParams) {
	url := "http://127.0.0.1/api/v1/notify"
	pubkey := hex.EncodeToString(p.LspClient().Node().PrivateKey().PubKey().SerializeCompressed())

	log.Printf("Client pubkey: %s", pubkey)

	count_subscriptions := func() (uint64, error) {
		row := p.lsp.PostgresBackend().Pool().QueryRow(
			p.h.Ctx,
			`SELECT COUNT(*)
		 	 FROM public.notification_subscriptions
		 	 WHERE pubkey = $1 AND url = $2`,
			pubkey,
			url,
		)

		var count uint64
		if err := row.Scan(&count); err != nil {
			p.h.T.Fatalf("Failed to query subscriptions: %v", err)
			return 0, err
		}

		return count, nil
	}

	SubscribeNotifications(p.lsp, p.LspClient(), url, false)

	// Verify that we have subscribed to the webhook
	count, err := count_subscriptions()
	assert.Nil(p.h.T, err)
	assert.Equal(p.h.T, count, 1)

	UnsubscribeNotifications(p.lsp, p.LspClient(), url, false)

	// Verify that we have unsubscribed from the webhook
	count, err = count_subscriptions()
	assert.Nil(p.h.T, err)
	assert.Equal(p.h.T, count, 0)
}
