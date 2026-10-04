package siwc

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type idClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience any    `json:"aud"`
	Email    string `json:"email"`
	Nonce    string `json:"nonce"`
	Expires  int64  `json:"exp"`
}

func (c idClaims) audienceOK(want string) bool {
	switch v := c.Audience.(type) {
	case string:
		return v == want
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

func verifyIDToken(raw, issuer, audience, nonce string, httpc *http.Client, now time.Time) (idClaims, error) {
	if httpc == nil {
		httpc = http.DefaultClient
	}
	if now.IsZero() {
		now = time.Now()
	}
	header, payload, sig, err := splitJWT(raw)
	if err != nil {
		return idClaims{}, err
	}
	var head struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(header, &head); err != nil {
		return idClaims{}, err
	}
	if head.Alg != "RS256" {
		return idClaims{}, fmt.Errorf("siwc: id_token alg %q, want RS256", head.Alg)
	}
	var claims idClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return idClaims{}, err
	}
	if claims.Issuer != issuer {
		return idClaims{}, fmt.Errorf("siwc: id_token issuer %q, want %q", claims.Issuer, issuer)
	}
	if !claims.audienceOK(audience) {
		return idClaims{}, fmt.Errorf("siwc: id_token audience does not include issued client id")
	}
	if claims.Nonce != nonce {
		return idClaims{}, fmt.Errorf("siwc: id_token nonce mismatch")
	}
	if claims.Subject == "" {
		return idClaims{}, fmt.Errorf("siwc: id_token missing sub")
	}
	if claims.Expires != 0 && now.Unix() >= claims.Expires {
		return idClaims{}, fmt.Errorf("siwc: id_token expired")
	}
	pub, err := fetchJWKSKey(httpc, issuer, head.Kid)
	if err != nil {
		return idClaims{}, err
	}
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sha256sum(raw[:strings.LastIndex(raw, ".")]), sig); err != nil {
		return idClaims{}, fmt.Errorf("siwc: id_token signature: %w", err)
	}
	return claims, nil
}

func sha256sum(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func splitJWT(raw string) (header, payload, sig []byte, err error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, nil, nil, fmt.Errorf("siwc: id_token is not a JWT")
	}
	header, err = base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, nil, err
	}
	payload, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, nil, err
	}
	sig, err = base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, nil, err
	}
	return header, payload, sig, nil
}

func fetchJWKSKey(httpc *http.Client, issuer, kid string) (*rsa.PublicKey, error) {
	jwksURL, err := discoverJWKS(httpc, issuer)
	if err != nil {
		return nil, err
	}
	res, err := httpc.Get(jwksURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("siwc: jwks http %d", res.StatusCode)
	}
	var set jwks
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, err
	}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		if kid != "" && k.Kid != "" && k.Kid != kid {
			continue
		}
		return jwkRSA(k)
	}
	return nil, fmt.Errorf("siwc: jwks missing RS256 key")
}

func discoverJWKS(httpc *http.Client, issuer string) (string, error) {
	res, err := httpc.Get(strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("siwc: discovery http %d", res.StatusCode)
	}
	var doc struct {
		JWKS string `json:"jwks_uri"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.JWKS == "" {
		return strings.TrimRight(issuer, "/") + "/.well-known/jwks.json", nil
	}
	return doc.JWKS, nil
}

func jwkRSA(k jwk) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	var exp int
	if len(e) <= 8 {
		buf := make([]byte, 8)
		copy(buf[8-len(e):], e)
		exp = int(binary.BigEndian.Uint64(buf))
	}
	if exp == 0 {
		return nil, fmt.Errorf("siwc: bad jwk exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}, nil
}
