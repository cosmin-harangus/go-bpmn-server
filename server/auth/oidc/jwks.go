package oidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
)

// keySet holds parsed public keys indexed by key ID.
type keySet map[string]any // value is *rsa.PublicKey or *ecdsa.PublicKey

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	// RSA fields
	N string `json:"n"`
	E string `json:"e"`
	// EC fields
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func fetchJWKS(client *http.Client, url string) (keySet, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read jwks body: %w", err)
	}
	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}

	ks := make(keySet, len(jwks.Keys))
	for _, k := range jwks.Keys {
		key, err := parseJWK(k)
		if err != nil {
			return nil, fmt.Errorf("parse key %q: %w", k.Kid, err)
		}
		ks[k.Kid] = key
	}
	return ks, nil
}

func parseJWK(k jwk) (any, error) {
	switch k.Kty {
	case "RSA":
		return parseRSA(k)
	case "EC":
		return parseEC(k)
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

func parseRSA(k jwk) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	e := int(new(big.Int).SetBytes(eb).Int64())
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}, nil
}

func parseEC(k jwk) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve %q", k.Crv)
	}
	xb, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yb, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(xb), Y: new(big.Int).SetBytes(yb)}, nil
}
