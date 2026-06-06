package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const (
	siteLogoDownloadMaxBytes = 300 * 1024
	siteLogoDownloadTimeout  = 10 * time.Second
)

// DownloadSiteLogo 下载远程 logo，并返回与现有 site_logo 设置兼容的 data URL。
func (s *SettingService) DownloadSiteLogo(ctx context.Context, rawURL string) (string, error) {
	normalizedURL, err := s.validateSiteLogoURL(rawURL)
	if err != nil {
		return "", infraerrors.BadRequest("INVALID_SITE_LOGO_URL", "invalid logo url").WithCause(err)
	}

	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:               siteLogoDownloadTimeout,
		ResponseHeaderTimeout: 5 * time.Second,
		ValidateResolvedIP:    s != nil && s.cfg != nil && s.cfg.Security.URLAllowlist.Enabled,
		AllowPrivateHosts:     s != nil && s.cfg != nil && s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
	if err != nil {
		return "", infraerrors.InternalServer("SITE_LOGO_HTTP_CLIENT_FAILED", "failed to create logo download client").WithCause(err)
	}

	downloadClient := *client
	downloadClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if req == nil || req.URL == nil {
			return fmt.Errorf("invalid redirect url")
		}
		_, err := s.validateSiteLogoURL(req.URL.String())
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if err != nil {
		return "", infraerrors.BadRequest("INVALID_SITE_LOGO_URL", "invalid logo url").WithCause(err)
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", "sub2api-logo-fetcher/1.0")

	resp, err := downloadClient.Do(req)
	if err != nil {
		return "", infraerrors.BadRequest("SITE_LOGO_DOWNLOAD_FAILED", "failed to download logo").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", infraerrors.BadRequest("SITE_LOGO_DOWNLOAD_FAILED", fmt.Sprintf("logo url returned status %d", resp.StatusCode))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, siteLogoDownloadMaxBytes+1))
	if err != nil {
		return "", infraerrors.BadRequest("SITE_LOGO_DOWNLOAD_FAILED", "failed to read logo response").WithCause(err)
	}
	if len(data) == 0 {
		return "", infraerrors.BadRequest("SITE_LOGO_EMPTY", "logo file is empty")
	}
	if len(data) > siteLogoDownloadMaxBytes {
		return "", infraerrors.BadRequest("SITE_LOGO_TOO_LARGE", "logo file exceeds 300KB limit")
	}

	contentType, err := normalizeSiteLogoContentType(resp.Header.Get("Content-Type"), data)
	if err != nil {
		return "", err
	}

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (s *SettingService) validateSiteLogoURL(rawURL string) (string, error) {
	allowInsecureHTTP := false
	allowPrivateHosts := false
	if s != nil && s.cfg != nil {
		allowInsecureHTTP = s.cfg.Security.URLAllowlist.AllowInsecureHTTP
		allowPrivateHosts = s.cfg.Security.URLAllowlist.AllowPrivateHosts
	}
	return urlvalidator.ValidateHTTPURL(rawURL, allowInsecureHTTP, urlvalidator.ValidationOptions{
		AllowPrivate: allowPrivateHosts,
	})
}

func normalizeSiteLogoContentType(header string, data []byte) (string, error) {
	contentType := strings.ToLower(strings.TrimSpace(header))
	if before, _, ok := strings.Cut(contentType, ";"); ok {
		contentType = strings.TrimSpace(before)
	}

	if contentType == "" || isGenericBinaryContentType(contentType) || !strings.HasPrefix(contentType, "image/") {
		detected := strings.ToLower(http.DetectContentType(data))
		if before, _, ok := strings.Cut(detected, ";"); ok {
			detected = strings.TrimSpace(before)
		}
		if strings.HasPrefix(detected, "image/") || looksLikeSVG(data) {
			contentType = detected
		}
	}
	if looksLikeSVG(data) && isSVGCompatibleContentType(contentType) {
		contentType = "image/svg+xml"
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", infraerrors.BadRequest("SITE_LOGO_INVALID_TYPE", "logo url must return an image file")
	}
	return contentType, nil
}

func isGenericBinaryContentType(contentType string) bool {
	switch contentType {
	case "application/octet-stream", "binary/octet-stream":
		return true
	default:
		return false
	}
}

func isSVGCompatibleContentType(contentType string) bool {
	switch contentType {
	case "", "application/octet-stream", "text/plain", "text/xml", "application/xml":
		return true
	default:
		return false
	}
}

func looksLikeSVG(data []byte) bool {
	trimmed := strings.TrimPrefix(strings.TrimSpace(string(data)), "\ufeff")
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "<?xml") {
		if idx := strings.Index(trimmed, "?>"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[idx+2:])
		}
	}
	return strings.HasPrefix(strings.ToLower(trimmed), "<svg")
}
