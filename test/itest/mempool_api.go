package itest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
)

type RecommendedFeesResponse struct {
	FastestFee  float64 `json:"fastestFee"`
	HalfHourFee float64 `json:"halfHourFee"`
	HourFee     float64 `json:"hourFee"`
	EconomyFee  float64 `json:"economyFee"`
	MinimumFee  float64 `json:"minimumFee"`
}

type mempoolApi struct {
	addr string
	t    *testing.T
	fees *RecommendedFeesResponse
	lis  net.Listener
}

// getPort finds a free TCP port by briefly binding to port 0 and releasing it.
func getPort() (uint32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return uint32(l.Addr().(*net.TCPAddr).Port), nil
}

func NewMempoolApi(t *testing.T) *mempoolApi {
	port, err := getPort()
	if err != nil {
		t.Fatalf("Failed to get port for mempool api: %v", err)
	}

	return &mempoolApi{
		addr: fmt.Sprintf("127.0.0.1:%d", port),
		t:    t,
		fees: &RecommendedFeesResponse{
			MinimumFee:  1,
			EconomyFee:  1,
			HourFee:     1,
			HalfHourFee: 1,
			FastestFee:  1,
		},
	}
}

func (m *mempoolApi) Address() string {
	return fmt.Sprintf("http://%s/api/v1/", m.addr)
}

func (m *mempoolApi) SetFees(fees *RecommendedFeesResponse) {
	m.fees = fees
}

func (m *mempoolApi) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/fees/recommended", func(w http.ResponseWriter, r *http.Request) {
		j, err := json.Marshal(m.fees)
		if err != nil {
			m.t.Fatalf("Failed to marshal mempool fees: %v", err)
		}
		_, err = w.Write(j)
		if err != nil {
			m.t.Fatalf("Failed to write mempool response: %v", err)
		}
	})
	lis, err := net.Listen("tcp", m.addr)
	if err != nil {
		m.t.Fatalf("failed to start mempool api: %v", err)
	}

	m.lis = lis
	m.t.Cleanup(func() { m.Stop() })

	go http.Serve(lis, mux)
}

func (m *mempoolApi) Stop() error {
	lis := m.lis
	if lis == nil {
		return nil
	}

	m.lis = nil
	return lis.Close()
}
