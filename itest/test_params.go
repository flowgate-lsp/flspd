package itest

import (
	"testing"

	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/itest/lntest"
)

type LspFunc func(h *lntest.TestHarness, m *lntest.Miner, mem *mempoolApi, c *config.NodeConfig) LspNode
type ClientFunc func(h *lntest.TestHarness, m *lntest.Miner) LspClient

type testParams struct {
	t          *testing.T
	h          *lntest.TestHarness
	m          *lntest.Miner
	mem        *mempoolApi
	c          LspClient
	lsp        LspNode
	lspFunc    LspFunc
	clientFunc ClientFunc
}

func (h *testParams) T() *testing.T {
	return h.t
}

func (h *testParams) Miner() *lntest.Miner {
	return h.m
}

func (h *testParams) Mempool() *mempoolApi {
	return h.mem
}

func (h *testParams) Lsp() LspNode {
	return h.lsp
}

func (h *testParams) Harness() *lntest.TestHarness {
	return h.h
}

func (h *testParams) LspClient() LspClient {
	return h.c
}
