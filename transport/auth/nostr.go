package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

const (
	KindHttpAuth = 27235

	// authValidityWindowSeconds must match the +/-60s "Verify Timestamp"
	// check below. replayCacheTTLSeconds is kept longer than double that
	// window so a seen event can never expire from the cache while its
	// timestamp would still otherwise be accepted.
	authValidityWindowSeconds = 60
	replayCacheTTLSeconds     = 3 * authValidityWindowSeconds
)

// seenEvents is a short-lived replay cache: NIP-98 auth events are only
// valid for a narrow timestamp window, so remembering event IDs a bit
// longer than that window is enough to reject replays without unbounded
// growth. Keyed by the event's recomputed canonical ID (event.GetID()),
// never the client-supplied ID field — go-nostr's CheckSignature does not
// itself validate that field against the signed content, so trusting it
// here would let an attacker dodge the replay cache by mutating it.
var (
	seenEventsMu sync.Mutex
	seenEvents   = make(map[string]int64)
)

func checkAndRecordReplay(eventID string, nowUnix int64) error {
	seenEventsMu.Lock()
	defer seenEventsMu.Unlock()

	for id, seenAt := range seenEvents {
		if nowUnix-seenAt > replayCacheTTLSeconds {
			delete(seenEvents, id)
		}
	}

	if _, exists := seenEvents[eventID]; exists {
		return fmt.Errorf("event already used (replay)")
	}
	seenEvents[eventID] = nowUnix
	return nil
}

// NostrAuth represents the authenticated session
type NostrAuth struct {
	PubKey string
}

// VerifyNostrAuth verifies a NIP-98 authentication header.
// Expected header: "Authorization: Nostr <base64_event>"
func VerifyNostrAuth(r *http.Request) (*NostrAuth, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Fallback to query parameter
		vals := r.URL.Query()
		// If proxied, sometimes r.URL.Query() doesn't parse correctly on upgrades
		if len(vals) == 0 && strings.Contains(r.RequestURI, "?") {
			parts := strings.Split(r.RequestURI, "?")
			if len(parts) > 1 {
				for _, pair := range strings.Split(parts[1], "&") {
					kv := strings.Split(pair, "=")
					if len(kv) == 2 && kv[0] == "auth" {
						authHeader = kv[1]
						break
					}
				}
			}
		} else {
			authHeader = vals.Get("auth")
		}

		if authHeader == "" {
			return nil, fmt.Errorf("missing Authorization header or auth query parameter")
		}
		
		// If it's a query param, it might not have the "Nostr " prefix, let's normalize
		if !strings.HasPrefix(strings.ToLower(authHeader), "nostr ") {
			authHeader = "Nostr " + authHeader
		}
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "nostr" {
		return nil, fmt.Errorf("invalid Authorization header format")
	}

	eventJson, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 event: %v", err)
	}

	var event nostr.Event
	if err := json.Unmarshal(eventJson, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nostr event: %v", err)
	}

	// 1. Verify Kind
	if event.Kind != KindHttpAuth {
		return nil, fmt.Errorf("invalid event kind: %d", event.Kind)
	}

	// 2. Verify Signature
	ok, err := event.CheckSignature()
	if err != nil || !ok {
		return nil, fmt.Errorf("invalid event signature")
	}

	// 3. Verify Timestamp (+/- 60s)
	now := time.Now().Unix()
	if int64(event.CreatedAt) < now-authValidityWindowSeconds || int64(event.CreatedAt) > now+authValidityWindowSeconds {
		return nil, fmt.Errorf("event timestamp expired or too far in future")
	}

	// 3b. Verify this exact event hasn't been used before (replay
	// protection). Must come after signature verification so the ID we
	// dedup on is backed by a valid signature, but GetID() recomputes it
	// from content regardless, so it can't be spoofed by the client.
	if err := checkAndRecordReplay(event.GetID(), now); err != nil {
		return nil, err
	}

	// 4. Verify URL and Method
	var urlOk, methodOk bool
	fullUrl := getFullUrl(r)

	for _, tag := range event.Tags {
		if len(tag) < 2 {
			continue
		}
		switch tag[0] {
		case "u":
			// Full match or match with scheme indifference (http vs https)
			if tag[1] == fullUrl {
				urlOk = true
			} else {
				// Also handle the case where the only difference is the protocol scheme (e.g. http vs https)
				// This is common behind proxies that terminate TLS but don't forward it correctly.
				s1 := strings.TrimPrefix(tag[1], "https://")
				s1 = strings.TrimPrefix(s1, "http://")
				s2 := strings.TrimPrefix(fullUrl, "https://")
				s2 = strings.TrimPrefix(s2, "http://")
				if s1 == s2 {
					urlOk = true
				}
			}
		case "m":
			if strings.EqualFold(tag[1], r.Method) {
				methodOk = true
			}
		}
	}

	if !urlOk {
		return nil, fmt.Errorf("event 'u' tag mismatch: expected %s, got %v", fullUrl, event.Tags.GetFirst([]string{"u"}))
	}
	if !methodOk {
		return nil, fmt.Errorf("event 'm' tag mismatch: expected %s", r.Method)
	}

	// 5. Verify Payload Hash for methods that carry a body. Without this, a
	// validly signed auth event for one request body could be replayed
	// (within the timestamp window, and before the same event ID was seen)
	// against a different body on the same URL/method.
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %v", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		if len(bodyBytes) > 0 {
			expectedHash := ComputePayloadHash(bodyBytes)
			payloadTag := event.Tags.Find("payload")
			if len(payloadTag) < 2 || payloadTag[1] != expectedHash {
				return nil, fmt.Errorf("event 'payload' tag mismatch or missing")
			}
		}
	}

	return &NostrAuth{
		PubKey: event.PubKey,
	}, nil
}

func getFullUrl(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	
	// Respect common proxy/load balancer forwarded headers
	if xForwardedProto := r.Header.Get("X-Forwarded-Proto"); xForwardedProto != "" {
		scheme = xForwardedProto
	} else if xForwardedSsl := r.Header.Get("X-Forwarded-Ssl"); xForwardedSsl == "on" {
		scheme = "https"
	} else if r.Header.Get("Front-End-Https") == "on" || r.Header.Get("X-Forwarded-Protocol") == "https" {
		scheme = "https"
	}

	// Prefer our custom X-Original-Proto if set by the Cloudflare Worker
	if xOrigProto := r.Header.Get("X-Original-Proto"); xOrigProto != "" {
		scheme = xOrigProto
	}

	host := r.Host
	if xForwardedHost := r.Header.Get("X-Forwarded-Host"); xForwardedHost != "" {
		host = xForwardedHost
	}
	// Prefer our custom X-Original-Host if set by the Cloudflare Worker
	if xOrigHost := r.Header.Get("X-Original-Host"); xOrigHost != "" {
		host = xOrigHost
	}

	path := r.URL.Path
	// Traefik appends X-Forwarded-Prefix when using stripprefix (e.g. /dashboard)
	if xForwardedPrefix := r.Header.Get("X-Forwarded-Prefix"); xForwardedPrefix != "" {
		path = xForwardedPrefix + path
	}

	// Normalize WS/WSS to HTTP/HTTPS for signature comparison
	// Some clients sign WSS events with HTTPS because they construct the URL generically
	if scheme == "ws" {
		scheme = "http"
	} else if scheme == "wss" {
		scheme = "https"
	}

	result := fmt.Sprintf("%s://%s%s", scheme, host, path)
	return strings.TrimSuffix(result, "/")
}

// ComputePayloadHash computes the SHA256 hex hash of a request body
func ComputePayloadHash(body []byte) string {
	hash := sha256.Sum256(body)
	return fmt.Sprintf("%x", hash)
}
