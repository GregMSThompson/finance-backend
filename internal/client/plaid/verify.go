package plaidclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// maxTokenAge bounds how old a Plaid-Verification JWS may be. Plaid signs
// tokens with iat; rejecting anything older guards against replay.
const maxTokenAge = 5 * time.Minute

// keyFetcher is the surface used to fetch a JWK from Plaid by kid. Satisfied
// by *Adapter; declared as an interface so the Verifier can be tested with a fake.
type keyFetcher interface {
	WebhookVerificationKeyGet(ctx context.Context, kid string) (PlaidWebhookKey, error)
}

// Verifier validates the Plaid-Verification JWS attached to every webhook.
// It caches public keys per-kid since Plaid rotates them rarely.
type Verifier struct {
	fetcher keyFetcher
	keys    map[string]*ecdsa.PublicKey
	mu      sync.RWMutex
}

func NewVerifier(fetcher keyFetcher) *Verifier {
	return &Verifier{
		fetcher: fetcher,
		keys:    make(map[string]*ecdsa.PublicKey),
	}
}

// Verify validates that verificationHeader is a Plaid-signed JWS whose
// request_body_sha256 claim matches the SHA-256 of body. Returns nil on success.
func (v *Verifier) Verify(ctx context.Context, verificationHeader string, body []byte) error {
	if verificationHeader == "" {
		return errors.New("missing Plaid-Verification header")
	}

	token, err := jwt.Parse(verificationHeader, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, errors.New("missing kid in jws header")
		}
		return v.getKey(ctx, kid)
	}, jwt.WithValidMethods([]string{"ES256"}))
	if err != nil {
		return fmt.Errorf("invalid jws signature: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("invalid jws claims")
	}

	iatRaw, ok := claims["iat"]
	if !ok {
		return errors.New("missing iat claim")
	}
	iatFloat, ok := iatRaw.(float64)
	if !ok {
		return errors.New("invalid iat claim")
	}
	iat := time.Unix(int64(iatFloat), 0)
	if time.Since(iat) > maxTokenAge {
		return errors.New("token too old")
	}

	expectedHash, ok := claims["request_body_sha256"].(string)
	if !ok {
		return errors.New("missing request_body_sha256 claim")
	}
	actual := sha256.Sum256(body)
	if expectedHash != hex.EncodeToString(actual[:]) {
		return errors.New("request body hash mismatch")
	}

	return nil
}

// getKey returns the cached *ecdsa.PublicKey for kid, fetching and caching it
// on miss. Keys are stable for long periods so we don't bother with TTL eviction.
func (v *Verifier) getKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	v.mu.RUnlock()
	if ok {
		return key, nil
	}

	jwk, err := v.fetcher.WebhookVerificationKeyGet(ctx, kid)
	if err != nil {
		return nil, err
	}
	key, err = buildPublicKey(jwk)
	if err != nil {
		return nil, err
	}

	v.mu.Lock()
	v.keys[kid] = key
	v.mu.Unlock()
	return key, nil
}

func buildPublicKey(jwk PlaidWebhookKey) (*ecdsa.PublicKey, error) {
	if jwk.Kty != "EC" || jwk.Crv != "P-256" {
		return nil, fmt.Errorf("unsupported key type: kty=%s crv=%s", jwk.Kty, jwk.Crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}