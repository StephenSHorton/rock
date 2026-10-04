package siwc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func randomString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func newPKCE() (verifier, challenge string, err error) {
	verifier, err = randomString(32)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func newStateNonce() (state, nonce string, err error) {
	state, err = randomString(16)
	if err != nil {
		return "", "", err
	}
	nonce, err = randomString(16)
	if err != nil {
		return "", "", fmt.Errorf("nonce: %w", err)
	}
	return state, nonce, nil
}
