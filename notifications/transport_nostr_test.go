package notifications

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingRelayServer is a minimal in-memory Nostr relay that just accepts
// WebSocket connections and counts how many real connection attempts it
// received — enough to prove the F-6 fix without a full protocol
// implementation (RelayPool.GetOrConnect only needs the handshake to
// succeed to consider a relay connected).
type countingRelayServer struct {
	addr        string
	connections int32
	server      *http.Server
}

func newCountingRelayServer(t *testing.T) *countingRelayServer {
	t.Helper()
	s := &countingRelayServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	s.server = &http.Server{Handler: mux}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s.addr = ln.Addr().String()

	go s.server.Serve(ln)
	t.Cleanup(func() { s.server.Close() })
	return s
}

func (s *countingRelayServer) handle(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&s.connections, 1)
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx := r.Context()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

func (s *countingRelayServer) url() string {
	return "ws://" + s.addr
}

func (s *countingRelayServer) connectionCount() int32 {
	return atomic.LoadInt32(&s.connections)
}

// TestRelayPool_ConcurrentGetOrConnect_SameURL is the regression test for
// F-6: GetOrConnect's load-check-connect-store had no per-URL lock, so N
// concurrent notifications to the same relay could each miss the cache,
// each dial, and each Store — all but the last connection leaked, never
// closed. With the singleflight fix, N concurrent calls for the same URL
// must result in exactly one real connection.
func TestRelayPool_ConcurrentGetOrConnect_SameURL(t *testing.T) {
	server := newCountingRelayServer(t)
	pool := NewRelayPool()

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := pool.GetOrConnect(ctx, server.url())
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		assert.NoError(t, err)
	}

	// Give the server a moment in case any stray connection attempt is
	// still in flight (shouldn't be, but avoids a flaky read of the
	// counter mid-accept).
	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, 1, server.connectionCount(), "20 concurrent GetOrConnect calls for the same URL must result in exactly one real connection")
}

// TestRelayPool_ConcurrentGetOrConnect_DifferentURLs confirms the
// singleflight fix doesn't over-serialize: different URLs must still
// connect independently and in parallel, not queue behind each other.
func TestRelayPool_ConcurrentGetOrConnect_DifferentURLs(t *testing.T) {
	serverA := newCountingRelayServer(t)
	serverB := newCountingRelayServer(t)
	pool := NewRelayPool()

	var wg sync.WaitGroup
	wg.Add(2)
	var errA, errB error
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, errA = pool.GetOrConnect(ctx, serverA.url())
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, errB = pool.GetOrConnect(ctx, serverB.url())
	}()
	wg.Wait()

	assert.NoError(t, errA)
	assert.NoError(t, errB)
	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, 1, serverA.connectionCount())
	assert.EqualValues(t, 1, serverB.connectionCount())
}
