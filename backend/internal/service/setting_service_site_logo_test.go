package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestSettingServiceDownloadSiteLogo(t *testing.T) {
	logoBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(logoBytes)
	}))
	defer server.Close()

	svc := newSiteLogoTestService()
	got, err := svc.DownloadSiteLogo(context.Background(), server.URL+"/logo.png")
	if err != nil {
		t.Fatalf("DownloadSiteLogo returned error: %v", err)
	}

	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("DownloadSiteLogo prefix = %q, want %q", got, prefix)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, prefix))
	if err != nil {
		t.Fatalf("decode data URL: %v", err)
	}
	if !bytes.Equal(decoded, logoBytes) {
		t.Fatalf("decoded logo bytes = %v, want %v", decoded, logoBytes)
	}
}

func TestSettingServiceDownloadSiteLogoRejectsNonImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not an image"))
	}))
	defer server.Close()

	_, err := newSiteLogoTestService().DownloadSiteLogo(context.Background(), server.URL+"/logo.txt")
	if infraerrors.Reason(err) != "SITE_LOGO_INVALID_TYPE" {
		t.Fatalf("DownloadSiteLogo reason = %q, want SITE_LOGO_INVALID_TYPE; err=%v", infraerrors.Reason(err), err)
	}
}

func TestSettingServiceDownloadSiteLogoRejectsOversizedImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(bytes.Repeat([]byte{1}, siteLogoDownloadMaxBytes+1))
	}))
	defer server.Close()

	_, err := newSiteLogoTestService().DownloadSiteLogo(context.Background(), server.URL+"/logo.png")
	if infraerrors.Reason(err) != "SITE_LOGO_TOO_LARGE" {
		t.Fatalf("DownloadSiteLogo reason = %q, want SITE_LOGO_TOO_LARGE; err=%v", infraerrors.Reason(err), err)
	}
}

func newSiteLogoTestService() *SettingService {
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	return NewSettingService(nil, cfg)
}
