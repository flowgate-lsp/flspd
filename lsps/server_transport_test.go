package lsps

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeCustomMessagesStream implements lnrpc.Lightning_SubscribeCustomMessagesClient
// with real logic only for Recv() (all subscribeAndListen calls) -- the
// embedded nil grpc.ClientStream satisfies the rest of the interface and
// would panic if any of those methods were ever called, which
// subscribeAndListen doesn't.
//
// Recv blocking on ctx.Done() (set via withContext, populated from the
// context subscribeAndListen actually passed to SubscribeCustomMessages --
// see the MockLndClient.On(...).Run(...) wiring below) matters, not just
// blocking forever: a real gRPC stream's Recv() returns once its context
// is canceled, which is exactly what subscribeAndListen's quit-channel
// watcher goroutine relies on to unblock a stream with nothing new to
// deliver. A fake that only ever plain-blocks would make any test
// depending on prompt shutdown pass or hang based on unrelated goroutine
// scheduling, not the behavior under test.
type fakeCustomMessagesStream struct {
	grpc.ClientStream
	messages []*lnrpc.CustomMessage
	recvErr  error
	idx      int
	ctx      context.Context
}

func (f *fakeCustomMessagesStream) withContext(ctx context.Context) *fakeCustomMessagesStream {
	f.ctx = ctx
	return f
}

func (f *fakeCustomMessagesStream) Recv() (*lnrpc.CustomMessage, error) {
	if f.idx < len(f.messages) {
		msg := f.messages[f.idx]
		f.idx++
		return msg, nil
	}
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	if f.ctx != nil {
		<-f.ctx.Done()
		return nil, status.Error(codes.Canceled, "context canceled")
	}
	select {}
}

func lspsMessage(peer []byte, payload interface{}) *lnrpc.CustomMessage {
	data, _ := json.Marshal(payload)
	return &lnrpc.CustomMessage{Peer: peer, Type: LspsMessageId, Data: data}
}

// TestHandleMessage_ValidRequest_RoutesAndReplies covers handleMessage's
// full round trip: unmarshal -> route -> marshal -> SendCustomMessage.
func TestHandleMessage_ValidRequest_RoutesAndReplies(t *testing.T) {
	mockLnd := new(MockLndClient)
	var sentData []byte
	mockLnd.On("SendCustomMessage", mock.Anything, mock.MatchedBy(func(req *lnrpc.SendCustomMessageRequest) bool {
		sentData = req.Data
		return req.Type == LspsMessageId
	})).Return(&lnrpc.SendCustomMessageResponse{}, nil)

	server := NewServer(Config{}, mockLnd, new(MockNodesService), new(MockOpeningService), new(MockLiquidityManager), new(MockNotificationService))
	msg := lspsMessage([]byte{0x01}, JsonRpcRequest{JsonRpc: "2.0", Method: "lsps0.list_protocols", Id: "req-1"})

	server.handleMessage(msg)

	require.NotEmpty(t, sentData)
	var resp JsonRpcResponse
	require.NoError(t, json.Unmarshal(sentData, &resp))
	assert.Nil(t, resp.Error)
	mockLnd.AssertCalled(t, "SendCustomMessage", mock.Anything, mock.Anything)
}

// TestHandleMessage_UnparsableJSON_DoesNotReply covers the malformed-input
// guard: handleMessage must give up quietly (no reply sent) rather than
// try to construct a response for a request it couldn't even parse an ID
// out of.
func TestHandleMessage_UnparsableJSON_DoesNotReply(t *testing.T) {
	mockLnd := new(MockLndClient)
	server := NewServer(Config{}, mockLnd, new(MockNodesService), new(MockOpeningService), new(MockLiquidityManager), new(MockNotificationService))
	msg := &lnrpc.CustomMessage{Peer: []byte{0x01}, Type: LspsMessageId, Data: []byte(`not-json`)}

	server.handleMessage(msg)

	mockLnd.AssertNotCalled(t, "SendCustomMessage", mock.Anything, mock.Anything)
}

// TestSubscribeAndListen_DispatchesLspsMessagesOnly covers the message-type
// filter (only LspsMessageId-typed custom messages are handled) and that
// a normal stream-end (context canceled) is treated as a clean shutdown,
// not an error to retry on.
func TestSubscribeAndListen_DispatchesLspsMessagesOnly(t *testing.T) {
	mockLnd := new(MockLndClient)
	var handledCount int32
	var mu sync.Mutex
	mockLnd.On("SendCustomMessage", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
		mu.Lock()
		handledCount++
		mu.Unlock()
	}).Return(&lnrpc.SendCustomMessageResponse{}, nil)

	stream := &fakeCustomMessagesStream{
		messages: []*lnrpc.CustomMessage{
			lspsMessage([]byte{0x01}, JsonRpcRequest{JsonRpc: "2.0", Method: "lsps0.list_protocols", Id: "1"}),
			{Peer: []byte{0x01}, Type: 99999, Data: []byte(`{}`)}, // not an LSPS message, must be ignored
		},
		recvErr: status.Error(codes.Canceled, "context canceled"),
	}
	mockLnd.On("SubscribeCustomMessages", mock.Anything, mock.Anything).Return(stream, nil)

	server := NewServer(Config{}, mockLnd, new(MockNodesService), new(MockOpeningService), new(MockLiquidityManager), new(MockNotificationService))
	server.quit = make(chan struct{})

	err := server.subscribeAndListen()
	require.NoError(t, err, "a Canceled recv error must be treated as a clean shutdown, not propagated as an error to retry on")

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return handledCount == 1
	}, time.Second, 10*time.Millisecond, "exactly the one LspsMessageId-typed message must be routed; the other type must be silently ignored")
}

// TestSubscribeAndListen_RecvErrorPropagates covers the retry-worthy error
// path: a non-Canceled Recv error must be returned so messageLoop's
// caller knows to back off and retry, not swallowed like a clean shutdown.
func TestSubscribeAndListen_RecvErrorPropagates(t *testing.T) {
	mockLnd := new(MockLndClient)
	stream := &fakeCustomMessagesStream{recvErr: errors.New("connection reset")}
	mockLnd.On("SubscribeCustomMessages", mock.Anything, mock.Anything).Return(stream, nil)

	server := NewServer(Config{}, mockLnd, new(MockNodesService), new(MockOpeningService), new(MockLiquidityManager), new(MockNotificationService))
	server.quit = make(chan struct{})

	err := server.subscribeAndListen()
	assert.Error(t, err)
}

// TestSubscribeAndListen_SubscribeErrorPropagates covers the case where
// the initial SubscribeCustomMessages call itself fails.
func TestSubscribeAndListen_SubscribeErrorPropagates(t *testing.T) {
	mockLnd := new(MockLndClient)
	mockLnd.On("SubscribeCustomMessages", mock.Anything, mock.Anything).Return(nil, assert.AnError)

	server := NewServer(Config{}, mockLnd, new(MockNodesService), new(MockOpeningService), new(MockLiquidityManager), new(MockNotificationService))
	server.quit = make(chan struct{})

	err := server.subscribeAndListen()
	assert.Error(t, err)
}

// TestStartStop_ReturnsPromptly covers Start/Stop/messageLoop's lifecycle:
// Stop must unblock messageLoop (and, transitively, subscribeAndListen's
// own quit-triggered context cancellation) rather than hang waiting for a
// stream that will never produce a normal Recv() error on its own.
func TestStartStop_ReturnsPromptly(t *testing.T) {
	mockLnd := new(MockLndClient)
	stream := &fakeCustomMessagesStream{} // blocks in Recv() until its context is canceled
	mockLnd.On("SubscribeCustomMessages", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		stream.withContext(args.Get(0).(context.Context))
	}).Return(stream, nil)

	server := NewServer(Config{}, mockLnd, new(MockNodesService), new(MockOpeningService), new(MockLiquidityManager), new(MockNotificationService))
	server.Start()

	done := make(chan struct{})
	go func() {
		server.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return promptly")
	}
}

// TestStop_IsIdempotent covers quitOnce: calling Stop twice must not panic
// on a double-close of the quit channel.
func TestStop_IsIdempotent(t *testing.T) {
	mockLnd := new(MockLndClient)
	stream := &fakeCustomMessagesStream{}
	mockLnd.On("SubscribeCustomMessages", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		stream.withContext(args.Get(0).(context.Context))
	}).Return(stream, nil)

	server := NewServer(Config{}, mockLnd, new(MockNodesService), new(MockOpeningService), new(MockLiquidityManager), new(MockNotificationService))
	server.Start()

	done := make(chan struct{})
	go func() {
		server.Stop()
		server.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("calling Stop twice must not hang or panic")
	}
}
