package itest

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlspdProcessSmoke(t *testing.T) {
	d := StartDevnet(t)

	pg, err := NewPostgresContainer(t.TempDir() + "/postgres.log")
	require.NoError(t, err)
	require.NoError(t, pg.Start(t.Context()))
	t.Cleanup(func() { pg.Stop(t.Context()) })

	p := NewFlspdProcess(t, d, pg, FlspdProcessOptions{JitEnabled: true})
	require.NoError(t, p.Start())
	t.Cleanup(func() { p.Stop() })

	conn, err := net.DialTimeout("tcp", p.ListenAddress(), time.Second)
	require.NoError(t, err)
	conn.Close()

	require.NoError(t, p.Restart())

	conn, err = net.DialTimeout("tcp", p.ListenAddress(), time.Second)
	require.NoError(t, err)
	conn.Close()
}
