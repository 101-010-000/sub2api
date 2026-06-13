package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

const (
	TouchPieHeaderName      = "x-touch-pie"
	TouchPieSignatureWindow = 5 * time.Minute
)

func VerifyTouchPieSignature(headerValue, gatewayAPIKey string, now time.Time) bool {
	headerValue = strings.TrimSpace(headerValue)
	gatewayAPIKey = strings.TrimSpace(gatewayAPIKey)
	if headerValue == "" || gatewayAPIKey == "" {
		return false
	}

	parts := strings.Split(headerValue, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return false
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || ts <= 0 {
		return false
	}
	signedAt := time.Unix(ts, 0)
	if now.IsZero() {
		now = time.Now()
	}
	if signedAt.Before(now.Add(-TouchPieSignatureWindow)) || signedAt.After(now.Add(TouchPieSignatureWindow)) {
		return false
	}
	nonce := strings.TrimSpace(parts[2])
	if nonce == "" || strings.Contains(nonce, ":") {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(got) != sha256.Size {
		return false
	}

	payload := "touch-pie:v1:" + parts[1] + ":" + nonce
	mac := hmac.New(sha256.New, []byte(gatewayAPIKey))
	_, _ = mac.Write([]byte(payload))
	want := mac.Sum(nil)
	return subtle.ConstantTimeCompare(got, want) == 1
}
