package itest

import (
	"crypto/rand"
	"log"
	"testing"

	rpc "github.com/flowgate-lsp/flspd/rpc"
	"github.com/stretchr/testify/assert"
)

var WorkingToken = "hello"

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

func AssertChannelCapacity(
	t *testing.T,
	outerAmountMloki uint64,
	capacityMloki uint64,
) {
	assert.Equal(t, ((outerAmountMloki/1000)+100000)*1000, capacityMloki)
}

func calculateInnerAmountMloki(lsp LspNode, outerAmountMloki uint64, params *rpc.OpeningFeeParams) uint64 {
	var fee uint64
	log.Printf("%+v", params)
	if params == nil {
		fee = outerAmountMloki * 40 / 10_000 / 1_000 * 1_000
		if fee < 2000000 {
			fee = 2000000
		}
	} else {
		fee = outerAmountMloki * uint64(params.Proportional) / 1_000_000 / 1_000 * 1_000
		if fee < params.MinMloki {
			fee = params.MinMloki
		}
	}

	if fee > outerAmountMloki {
		lsp.Harness().Fatalf("Fee is higher than amount")
	}

	log.Printf("outer: %v, fee: %v", outerAmountMloki, fee)
	return outerAmountMloki - fee
}

var publicChanAmount uint64 = 1000183
