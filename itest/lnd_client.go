package itest

import (
	"context"
	"flag"
	"sync"

	"github.com/flokiorg/flnd/lnwire"
	"github.com/flowgate-lsp/flspd/itest/lntest"
	"github.com/flowgate-lsp/flspd/itest/lntest/lnd"
)

var lndMobileExecutable = flag.String(
	"lndmobileexec", "", "full path to lnd mobile binary",
)

type lndLspClient struct {
	name    string
	harness *lntest.TestHarness
	node    *lntest.LndNode
	cancel  context.CancelFunc
	mtx     sync.Mutex
}

func newLndLspClient(h *lntest.TestHarness, m *lntest.Miner, name string) LspClient {
	lnd := lntest.NewLndNodeFromBinary(h, m, name, *lndMobileExecutable,
		"--protocol.zero-conf",
		"--protocol.option-scid-alias",
		"--bitcoin.defaultchanconfs=0",
	)

	c := &lndLspClient{
		name:    name,
		harness: h,
		node:    lnd,
	}
	h.AddStoppable(c)
	return c
}

func (c *lndLspClient) Name() string {
	return c.name
}

func (c *lndLspClient) Harness() *lntest.TestHarness {
	return c.harness
}

func (c *lndLspClient) Node() lntest.LightningNode {
	return c.node
}

func (c *lndLspClient) Start() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.node.IsStarted() {
		return
	}

	c.node.Start()

	ctx, cancel := context.WithCancel(c.harness.Ctx)
	c.cancel = cancel
	go c.startChannelAcceptor(ctx)
}

func (c *lndLspClient) Stop() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// Stop the channel acceptor
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	return c.node.Stop()
}

func (c *lndLspClient) ResetHtlcAcceptor() {

}

func (c *lndLspClient) SetHtlcAcceptor(totalMloki uint64) {
	// No need for a htlc acceptor in the LND client
}

func (c *lndLspClient) startChannelAcceptor(ctx context.Context) error {
	client, err := c.node.LightningClient().ChannelAcceptor(ctx)
	if err != nil {
		c.harness.T.Fatalf("%s: failed to create channel acceptor: %v", c.name, err)
	}

	for {
		request, err := client.Recv()
		if err != nil {
			return err
		}

		private := request.ChannelFlags&uint32(lnwire.FFAnnounceChannel) == 0
		resp := &lnd.ChannelAcceptResponse{
			PendingChanId: request.PendingChanId,
			Accept:        private,
		}
		if request.WantsZeroConf {
			resp.MinAcceptDepth = 0
			resp.ZeroConf = true
		}

		err = client.Send(resp)
		if err != nil {
			c.harness.T.Fatalf("%s: failed to send acceptor response: %v", c.name, err)
		}
	}
}
