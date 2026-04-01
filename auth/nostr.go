package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

const (
	KindHttpAuth = 27235
)

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
	if int64(event.CreatedAt) < now-60 || int64(event.CreatedAt) > now+60 {
		return nil, fmt.Errorf("event timestamp expired or too far in future")
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
			if tag[1] == fullUrl {
				urlOk = true
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

	// 5. Verify Payload Hash (if body exists)
	// Skip for now to support simple GETs, but should be enforced for POST
	// if r.Method == http.MethodPost {
	//    ... check hash ...
	// }

	return &NostrAuth{
		PubKey: event.PubKey,
	}, nil
}

func getFullUrl(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	
	// Respect Cloudflare/Proxy forwarded headers
	if xForwardedProto := r.Header.Get("X-Forwarded-Proto"); xForwardedProto != "" {
		scheme = xForwardedProto
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
