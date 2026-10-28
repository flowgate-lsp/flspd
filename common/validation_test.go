package common

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	require.NotNil(t, ip, "failed to parse test IP %q", s)
	return ip
}

func TestValidateWebhookUrl(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errIs   string
	}{
		{name: "valid https public host", url: "https://example.com/webhook", wantErr: false},
		{name: "http on localhost is allowed (dev)", url: "http://localhost/webhook", wantErr: false},
		{name: "http on 127.0.0.1 is allowed (dev)", url: "http://127.0.0.1/webhook", wantErr: false},
		{name: "plain http on a public host is rejected", url: "http://example.com/webhook", wantErr: true, errIs: "unsupported_protocol"},
		{name: "not a url at all", url: "://not-a-url", wantErr: true},
		{name: "https with no hostname", url: "https:///path", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookUrl(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errIs != "" {
					assert.Contains(t, err.Error(), tt.errIs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestIsPrivateIP is the SSRF matrix: a webhook URL resolving to any of
// these ranges must be rejected so an attacker can't register a webhook
// that makes flspd call back into its own private network.
func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.15.255.255", false}, // just outside the 172.16/12 range
		{"172.32.0.1", false},     // just outside the 172.16/12 range
		{"192.168.1.1", true},
		{"192.169.1.1", false},
		{"169.254.1.1", true}, // link-local
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"::1", true},       // IPv6 loopback
		{"fd00::1", true},   // IPv6 unique local
		{"2001:db8::1", false}, // IPv6 public documentation range
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := mustParseIP(t, tt.ip)
			assert.Equal(t, tt.private, isPrivateIP(ip), "isPrivateIP(%s)", tt.ip)
		})
	}
}

func TestParsePrivateKey(t *testing.T) {
	validHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	require.Len(t, validHex, 64)

	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{name: "empty key", key: "", wantErr: true},
		{name: "valid 64-char hex", key: validHex, want: validHex},
		{name: "too short", key: "abcd", wantErr: true},
		{name: "too long", key: validHex + "ab", wantErr: true},
		{name: "malformed nsec", key: "nsec1notvalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePrivateKey(tt.key)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
