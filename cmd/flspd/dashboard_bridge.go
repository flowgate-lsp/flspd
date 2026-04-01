package main

import (
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/dashboard"
	"github.com/flowgate-lsp/flspd/notifications"
)

type DashboardNotificationBridge struct {
	inner   *notifications.NotificationService
	dashSvc *dashboard.DashboardService
}

func NewDashboardNotificationBridge(inner *notifications.NotificationService, dashSvc *dashboard.DashboardService) *DashboardNotificationBridge {
	return &DashboardNotificationBridge{
		inner:   inner,
		dashSvc: dashSvc,
	}
}

func (b *DashboardNotificationBridge) NotifyWebhookRegistered(pubkey string, webhook *common.Lsps5Webhook) bool {
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
