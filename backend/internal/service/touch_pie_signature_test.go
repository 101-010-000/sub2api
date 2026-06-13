package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

func TestVerifyTouchPieSignature(t *testing.T) {
	now := time.Unix(1700000000, 0)
	key := "sk-test"
	nonce := "nonce-1"
	header := signTouchPieTestHeader(key, now.Unix(), nonce)

	if !VerifyTouchPieSignature(header, key, now) {
		t.Fatal("expected valid signature")
	}
	if VerifyTouchPieSignature(header, "other-key", now) {
		t.Fatal("expected wrong key to fail")
	}
	if VerifyTouchPieSignature(header[:len(header)-2]+"xx", key, now) {
		t.Fatal("expected tampered signature to fail")
	}
	if VerifyTouchPieSignature(header, key, now.Add(6*time.Minute)) {
		t.Fatal("expected expired signature to fail")
	}
	if VerifyTouchPieSignature("v1.bad.nonce.sig", key, now) {
		t.Fatal("expected malformed signature to fail")
	}
}

func signTouchPieTestHeader(key string, ts int64, nonce string) string {
	tsText := strconv.FormatInt(ts, 10)
	payload := "touch-pie:v1:" + tsText + ":" + nonce
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(payload))
	return "v1." + tsText + "." + nonce + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
