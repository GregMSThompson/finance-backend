package plaidclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// --- helpers ---

type fakeFetcher struct {
	keys  map[string]PlaidWebhookKey
	err   error
	calls int
}

func (f *fakeFetcher) WebhookVerificationKeyGet(ctx context.Context, kid string) (PlaidWebhookKey, error) {
	f.calls++
	if f.err != nil {
		return PlaidWebhookKey{}, f.err
	}
	if k, ok := f.keys[kid]; ok {
		return k, nil
	}
	return PlaidWebhookKey{}, errors.New("kid not found")
}

func newTestKey(t *testing.T, kid string) (*ecdsa.PrivateKey, PlaidWebhookKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return priv, PlaidWebhookKey{
		Kid: kid,
		Alg: "ES256",
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(priv.PublicKey.X.Bytes()),
		Y:   base64.RawURLEncoding.EncodeToString(priv.PublicKey.Y.Bytes()),
	}
}

func signToken(t *testing.T, priv *ecdsa.PrivateKey, kid string, body []byte, iat time.Time) string {
	t.Helper()
	hash := sha256.Sum256(body)
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iat":                 iat.Unix(),
		"request_body_sha256": hex.EncodeToString(hash[:]),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

// --- tests ---

func TestVerifySuccess(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	v := NewVerifier(&fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}})

	body := []byte(`{"webhook_type":"TRANSACTIONS","item_id":"abc"}`)
	token := signToken(t, priv, jwk.Kid, body, time.Now())

	if err := v.Verify(context.Background(), token, body); err != nil {
		t.Fatalf("Verify returned %v, want nil", err)
	}
}

func TestVerifyMissingHeader(t *testing.T) {
	v := NewVerifier(&fakeFetcher{})
	err := v.Verify(context.Background(), "", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for missing header")
	}
}

func TestVerifyMissingKid(t *testing.T) {
	priv, _ := newTestKey(t, "kid-1")
	v := NewVerifier(&fakeFetcher{})

	body := []byte(`{}`)
	hash := sha256.Sum256(body)
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iat":                 time.Now().Unix(),
		"request_body_sha256": hex.EncodeToString(hash[:]),
	})
	// deliberately do not set kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	err = v.Verify(context.Background(), signed, body)
	if err == nil || !strings.Contains(err.Error(), "kid") {
		t.Fatalf("expected kid error, got %v", err)
	}
}

func TestVerifyBadSignature(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	otherPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}
	v := NewVerifier(&fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}})

	body := []byte(`{}`)
	// sign with otherPriv but ship under priv's kid — signature won't verify
	token := signToken(t, otherPriv, jwk.Kid, body, time.Now())
	_ = priv

	err = v.Verify(context.Background(), token, body)
	if err == nil {
		t.Fatal("expected signature error, got nil")
	}
}

func TestVerifyBodyHashMismatch(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	v := NewVerifier(&fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}})

	signedBody := []byte(`{"a":1}`)
	tamperedBody := []byte(`{"a":2}`)
	token := signToken(t, priv, jwk.Kid, signedBody, time.Now())

	err := v.Verify(context.Background(), token, tamperedBody)
	if err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("expected hash mismatch error, got %v", err)
	}
}

func TestVerifyTokenTooOld(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	v := NewVerifier(&fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}})

	body := []byte(`{}`)
	old := time.Now().Add(-10 * time.Minute)
	token := signToken(t, priv, jwk.Kid, body, old)

	err := v.Verify(context.Background(), token, body)
	if err == nil || !strings.Contains(err.Error(), "old") {
		t.Fatalf("expected too-old error, got %v", err)
	}
}

func TestVerifyCacheHitAvoidsSecondFetch(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	fetcher := &fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}}
	v := NewVerifier(fetcher)

	body := []byte(`{}`)
	for i := 0; i < 3; i++ {
		token := signToken(t, priv, jwk.Kid, body, time.Now())
		if err := v.Verify(context.Background(), token, body); err != nil {
			t.Fatalf("Verify #%d returned %v", i, err)
		}
	}

	if fetcher.calls != 1 {
		t.Fatalf("expected 1 fetch (cached), got %d", fetcher.calls)
	}
}

func TestVerifyCacheEvictsExpiredKey(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	expired := time.Now().Add(-1 * time.Hour).Unix()
	jwk.ExpiredAt = &expired
	fetcher := &fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}}
	v := NewVerifier(fetcher)

	body := []byte(`{}`)
	for i := 0; i < 3; i++ {
		token := signToken(t, priv, jwk.Kid, body, time.Now())
		if err := v.Verify(context.Background(), token, body); err != nil {
			t.Fatalf("Verify #%d returned %v", i, err)
		}
	}

	// expired_at is always in the past so every call should evict and refetch.
	if fetcher.calls != 3 {
		t.Fatalf("expected 3 fetches (no caching of expired key), got %d", fetcher.calls)
	}
}

func TestVerifyUnsupportedKeyType(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	jwk.Kty = "RSA" // unsupported
	v := NewVerifier(&fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}})

	body := []byte(`{}`)
	token := signToken(t, priv, jwk.Kid, body, time.Now())

	err := v.Verify(context.Background(), token, body)
	if err == nil || !strings.Contains(err.Error(), "unsupported key type") {
		t.Fatalf("expected unsupported key error, got %v", err)
	}
}

func TestVerifyFetcherError(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	v := NewVerifier(&fakeFetcher{err: errors.New("plaid down")})

	body := []byte(`{}`)
	token := signToken(t, priv, jwk.Kid, body, time.Now())

	err := v.Verify(context.Background(), token, body)
	if err == nil {
		t.Fatal("expected fetcher error to propagate")
	}
}

func TestVerifyWrongAlgorithm(t *testing.T) {
	priv, jwk := newTestKey(t, "kid-1")
	v := NewVerifier(&fakeFetcher{keys: map[string]PlaidWebhookKey{jwk.Kid: jwk}})

	body := []byte(`{}`)
	// Build a token claiming none/HS256 — should be rejected via WithValidMethods.
	hash := sha256.Sum256(body)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iat":                 time.Now().Unix(),
		"request_body_sha256": hex.EncodeToString(hash[:]),
	})
	tok.Header["kid"] = jwk.Kid
	signed, err := tok.SignedString([]byte("symmetric-secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	_ = priv

	err = v.Verify(context.Background(), signed, body)
	if err == nil {
		t.Fatal("expected algorithm rejection")
	}
}