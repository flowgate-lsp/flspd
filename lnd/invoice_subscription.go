package lnd

import (
	"context"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/rs/zerolog/log"
)

// SubscribeInvoices subscribes to invoice updates from LND
func (c *LndClient) SubscribeInvoices() (<-chan *lightning.InvoiceUpdate, <-chan error, error) {
	updateChan := make(chan *lightning.InvoiceUpdate, 10)
	errChan := make(chan error, 1)

	stream, err := c.client.SubscribeInvoices(context.Background(), &lnrpc.InvoiceSubscription{
		AddIndex:    0, // Start from beginning (all invoices)
		SettleIndex: 0, // Start from beginning
	})
	if err != nil {
		close(updateChan)
		close(errChan)
		return updateChan, errChan, err
	}

	go func() {
		defer close(updateChan)
		defer close(errChan)

		for {
			invoice, err := stream.Recv()
			if err != nil {
				errChan <- err
				log.Warn().Err(err).Msg("LND: SubscribeInvoices stream error")
				return
			}

			// Convert LND invoice to our InvoiceUpdate type
			update := &lightning.InvoiceUpdate{
				State:          convertInvoiceState(invoice.State),
				PaymentRequest: invoice.PaymentRequest,
				AmountMloki:    uint64(invoice.Value),
				SettleDate:     time.Unix(invoice.SettleDate, 0),
			}

			updateChan <- update
		}
	}()

	return updateChan, errChan, nil
}

// convertInvoiceState converts LND invoice state to our InvoiceState type
func convertInvoiceState(state lnrpc.Invoice_InvoiceState) lightning.InvoiceState {
	switch state {
	case lnrpc.Invoice_OPEN:
		return lightning.InvoiceStateOpen
	case lnrpc.Invoice_SETTLED:
		return lightning.InvoiceStateSettled
	case lnrpc.Invoice_CANCELED:
		return lightning.InvoiceStateCanceled
	case lnrpc.Invoice_ACCEPTED:
		return lightning.InvoiceStateAccepted
	default:
		return lightning.InvoiceStateOpen
	}
}
