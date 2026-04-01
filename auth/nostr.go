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
		// Fallback to query parameter for WebSockets or other cases where headers are difficult
		authHeader = r.URL.Query().Get("auth")
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
	if xForwardedProto := r.Header.Get("X-Forwarded-Proto"); xForwardedProto != "" {
		scheme = xForwardedProto
	}

	host := r.Host
	return fmt.Sprintf("%s://%s%s", scheme, host, r.URL.Path)
}

// ComputePayloadHash computes the SHA256 hex hash of a request body
func ComputePayloadHash(body []byte) string {
	hash := sha256.Sum256(body)
	return fmt.Sprintf("%x", hash)
}
