package main

import (
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/notifications"
	transdashboard "github.com/flowgate-lsp/flspd/transport/dashboard"
)

type DashboardNotificationBridge struct {
	inner   *notifications.NotificationService
	dashSvc *transdashboard.Server
}

func NewDashboardNotificationBridge(inner *notifications.NotificationService, dashSvc *transdashboard.Server) *DashboardNotificationBridge {
	return &DashboardNotificationBridge{
		inner:   inner,
		dashSvc: dashSvc,
	}
}

func (b *DashboardNotificationBridge) Notify(pubkey string, paymenthash string, nodePrivKey interface{}) (bool, error) {
	// 1. Deliver to LSPS5 webhooks
	res, err := b.inner.Notify(pubkey, paymenthash, nodePrivKey)

	// 2. Broadcast to Dashboard WebSocket
	b.dashSvc.BroadcastEvent("payment_incoming", map[string]interface{}{
		"pubkey":       pubkey,
		"payment_hash": paymenthash,
	})

	return res, err
}

func (b *DashboardNotificationBridge) NotifyWebhookRegistered(pubkey string, webhook *core.Lsps5Webhook) bool {
	return b.inner.NotifyWebhookRegistered(pubkey, webhook)
}

func (b *DashboardNotificationBridge) NotifyOrderStateChanged(pubkey string, orderId string, state string, channelPoint string, errorMsg string) (bool, error) {
	// 1. Deliver to LSPS5 webhooks
	res, err := b.inner.NotifyOrderStateChanged(pubkey, orderId, state, channelPoint, errorMsg)

	// 2. Broadcast to Dashboard WebSocket
	b.dashSvc.BroadcastEvent("order_state_changed", map[string]interface{}{
		"order_id":      orderId,
		"state":         state,
		"channel_point": channelPoint,
		"error":         errorMsg,
	})

	return res, err
}

func (b *DashboardNotificationBridge) NotifyExpirySoon(pubkey string, timeout int, nodePrivKey interface{}, paymentHash string) (bool, error) {
	res, err := b.inner.NotifyExpirySoon(pubkey, timeout, nodePrivKey, paymentHash)

	b.dashSvc.BroadcastEvent("expiry_soon", map[string]interface{}{
		"pubkey":       pubkey,
		"timeout":      timeout,
		"payment_hash": paymentHash,
	})

	return res, err
}

func (b *DashboardNotificationBridge) NotifyLiquidityManagementRequest(pubkey string, nodePrivKey interface{}) (bool, error) {
	return b.inner.NotifyLiquidityManagementRequest(pubkey, nodePrivKey)
}

func (b *DashboardNotificationBridge) NotifyOnionMessageIncoming(pubkey string, nodePrivKey interface{}) (bool, error) {
	return b.inner.NotifyOnionMessageIncoming(pubkey, nodePrivKey)
}
