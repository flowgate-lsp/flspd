package notifications

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/flowgate-lsp/flspd/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOpeningStore implements common.OpeningStore with real logic only for
// GetLsps5Webhooks (all NotificationService needs); every other method is a
// no-op stub so the fake satisfies the (large) interface without pulling in
// a mocking framework for methods this package never calls.
type fakeOpeningStore struct {
	webhooks       map[string][]*common.Lsps5Webhook
	getWebhooksErr error
}

func newFakeOpeningStore() *fakeOpeningStore {
	return &fakeOpeningStore{webhooks: make(map[string][]*common.Lsps5Webhook)}
}

func (f *fakeOpeningStore) GetLsps5Webhooks(clientNodeId string) ([]*common.Lsps5Webhook, error) {
	if f.getWebhooksErr != nil {
		return nil, f.getWebhooksErr
	}
	return f.webhooks[clientNodeId], nil
}

func (f *fakeOpeningStore) GetFeeParamsSettings(string) ([]*common.OpeningFeeParamsSetting, error) {
	return nil, nil
}
func (f *fakeOpeningStore) SetFeeParams(string, time.Duration, common.OpeningFeeParams) error {
	return nil
}
func (f *fakeOpeningStore) RegisterLsps2Buy(*common.Lsps2BuyRegistration) error { return nil }
func (f *fakeOpeningStore) GetLsps2BuyRegistration(uint64) (*common.Lsps2BuyRegistration, error) {
	return nil, nil
}
func (f *fakeOpeningStore) RegisterLsps1Order(*common.Lsps1Order) error      { return nil }
func (f *fakeOpeningStore) GetLsps1Order(string) (*common.Lsps1Order, error) { return nil, nil }
func (f *fakeOpeningStore) GetLsps1OrderByPaymentInvoice(string) (*common.Lsps1Order, error) {
	return nil, nil
}
func (f *fakeOpeningStore) GetPaidLsps1Order(string) (*common.Lsps1Order, error) { return nil, nil }
func (f *fakeOpeningStore) GetPaidLsps1Orders(string) ([]*common.Lsps1Order, error) {
	return nil, nil
}
func (f *fakeOpeningStore) GetAllPaidLsps1Orders() ([]*common.Lsps1Order, error) { return nil, nil }
func (f *fakeOpeningStore) GetAllCreatedLsps1Orders() ([]*common.Lsps1Order, error) {
	return nil, nil
}
func (f *fakeOpeningStore) ListLsps1Orders(int, int) ([]*common.Lsps1Order, error) { return nil, nil }
func (f *fakeOpeningStore) UpdateLsps1OrderState(string, string, string, string) (bool, error) {
	return false, nil
}
func (f *fakeOpeningStore) UpdateLsps1OrderFunding(string, string, int) error { return nil }
func (f *fakeOpeningStore) RegisterLsps5Webhook(*common.Lsps5Webhook) error   { return nil }
func (f *fakeOpeningStore) RemoveLsps5Webhook(string, string) error           { return nil }
func (f *fakeOpeningStore) CountLsps1Orders(string, string, *time.Time) (int, error) {
	return 0, nil
}
func (f *fakeOpeningStore) CountLsps5Webhooks(string) (int, error) { return 0, nil }
func (f *fakeOpeningStore) GetReservedLiquidity() (uint64, error)  { return 0, nil }
func (f *fakeOpeningStore) ExpireStaleOrders() (int64, error)      { return 0, nil }

// fakeNotifStore implements Store in-memory, so processPending/Enqueue tests
// don't need a real database.
type fakeNotifStore struct {
	mu    sync.Mutex
	items map[int64]*NotificationQueueItem
	next  int64
}

func newFakeNotifStore() *fakeNotifStore {
	return &fakeNotifStore{items: make(map[int64]*NotificationQueueItem)}
}

func (f *fakeNotifStore) Register(context.Context, string, string, string) error { return nil }
func (f *fakeNotifStore) GetRegistrations(context.Context, string) ([]common.NotificationRegistration, error) {
	return nil, nil
}
func (f *fakeNotifStore) Unsubscribe(context.Context, string, string) error { return nil }
func (f *fakeNotifStore) RemoveExpired(context.Context, time.Time) error    { return nil }

func (f *fakeNotifStore) Enqueue(_ context.Context, pubkey, url, transport, method string, params interface{}) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.items[f.next] = &NotificationQueueItem{
		ID: f.next, Pubkey: pubkey, TargetURL: url, Transport: transport, Method: method, Params: params,
	}
	return f.next, nil
}

func (f *fakeNotifStore) GetPending(_ context.Context, limit int) ([]NotificationQueueItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []NotificationQueueItem
	for _, item := range f.items {
		if len(out) >= limit {
			break
		}
		out = append(out, *item)
	}
	return out, nil
}

func (f *fakeNotifStore) UpdateStatus(_ context.Context, id int64, status string, nextRetry time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if item, ok := f.items[id]; ok {
		item.Attempts++
		_ = status
		_ = nextRetry
		if status != "failed" {
			delete(f.items, id) // simulate leaving the retry window
		}
	}
	return nil
}

// stubTransport lets tests control success/failure per call without a real
// network dependency.
type stubTransport struct {
	transportType TransportType
	sendFunc      func(ctx context.Context, target, method string, params interface{}) error
	calls         int32
	mu            sync.Mutex
}

func (s *stubTransport) Type() TransportType { return s.transportType }
func (s *stubTransport) Send(ctx context.Context, target, method string, params interface{}) error {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return s.sendFunc(ctx, target, method, params)
}
func (s *stubTransport) callCount() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestNotify_DispatchesToAllWebhooksAndDedupsWithinWindow(t *testing.T) {
	store := newFakeOpeningStore()
	store.webhooks["peer1"] = []*common.Lsps5Webhook{
		{ClientNodeId: "peer1", AppName: "wallet-a", Url: "https://a.example", Transport: "push"},
		{ClientNodeId: "peer1", AppName: "wallet-b", Url: "https://b.example", Transport: "push"},
	}

	transport := &stubTransport{
		transportType: TransportTypePush,
		sendFunc:      func(context.Context, string, string, interface{}) error { return nil },
	}

	svc := NewNotificationService(store, nil, newFakeNotifStore())
	svc.AddTransport(transport)

	notified, err := svc.Notify("peer1", "hash1", nil)
	require.NoError(t, err)
	assert.True(t, notified)
	assert.EqualValues(t, 2, transport.callCount(), "should dispatch to both registered webhooks")

	// A second call for the same (pubkey, paymenthash) within the dedup
	// window should short-circuit without hitting the transport again.
	notified, err = svc.Notify("peer1", "hash1", nil)
	require.NoError(t, err)
	assert.True(t, notified)
	assert.EqualValues(t, 2, transport.callCount(), "recent notification should be deduped, not re-dispatched")
}

func TestNotify_NoWebhooksReturnsNotNotified(t *testing.T) {
	store := newFakeOpeningStore()
	svc := NewNotificationService(store, nil, newFakeNotifStore())

	notified, err := svc.Notify("unknown-peer", "hash1", nil)
	require.NoError(t, err)
	assert.False(t, notified)
}

func TestProcessPending_SuccessMarksCompleted(t *testing.T) {
	store := newFakeOpeningStore()
	notifStore := newFakeNotifStore()
	id, err := notifStore.Enqueue(context.Background(), "peer1", "https://a.example", "push", "lsps5.payment_incoming", nil)
	require.NoError(t, err)

	transport := &stubTransport{
		transportType: TransportTypePush,
		sendFunc:      func(context.Context, string, string, interface{}) error { return nil },
	}
	svc := NewNotificationService(store, nil, notifStore)
	svc.AddTransport(transport)

	svc.processPending(context.Background())

	notifStore.mu.Lock()
	_, stillPending := notifStore.items[id]
	notifStore.mu.Unlock()
	assert.False(t, stillPending, "a successfully delivered item should be marked completed and no longer pending")
}

func TestProcessPending_FailureBacksOff(t *testing.T) {
	store := newFakeOpeningStore()
	notifStore := newFakeNotifStore()
	id, err := notifStore.Enqueue(context.Background(), "peer1", "https://a.example", "push", "lsps5.payment_incoming", nil)
	require.NoError(t, err)

	transport := &stubTransport{
		transportType: TransportTypePush,
		sendFunc:      func(context.Context, string, string, interface{}) error { return assert.AnError },
	}
	svc := NewNotificationService(store, nil, notifStore)
	svc.AddTransport(transport)

	svc.processPending(context.Background())

	notifStore.mu.Lock()
	item, stillPending := notifStore.items[id]
	notifStore.mu.Unlock()
	require.True(t, stillPending, "a failed delivery should remain pending for retry")
	assert.EqualValues(t, 1, item.Attempts)
}
