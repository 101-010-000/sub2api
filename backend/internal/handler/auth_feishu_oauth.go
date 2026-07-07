package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/oauth"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
)

const (
	feishuOAuthCookiePath         = "/api/v1/auth/oauth/feishu"
	feishuOAuthStateCookieName    = "feishu_oauth_state"
	feishuOAuthRedirectCookie     = "feishu_oauth_redirect"
	feishuOAuthIntentCookieName   = "feishu_oauth_intent"
	feishuOAuthBindUserCookieName = "feishu_oauth_bind_user"
	feishuOAuthCookieMaxAgeSec    = 10 * 60
	feishuOAuthDefaultRedirectTo  = "/dashboard"
	feishuOAuthDefaultFrontendCB  = "/auth/feishu/callback"

	feishuOAuthMaxSubjectLen = 96
)

type feishuTokenResponse struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int64
	RefreshToken string
	Scope        string
}

type feishuTokenExchangeError struct {
	StatusCode          int
	ProviderCode        string
	ProviderMessage     string
	ProviderError       string
	ProviderDescription string
	Body                string
}

func (e *feishuTokenExchangeError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("token exchange status=%d", e.StatusCode)}
	for _, part := range []string{
		"code=" + strings.TrimSpace(e.ProviderCode),
		"msg=" + strings.TrimSpace(e.ProviderMessage),
		"error=" + strings.TrimSpace(e.ProviderError),
		"error_description=" + strings.TrimSpace(e.ProviderDescription),
	} {
		if !strings.HasSuffix(part, "=") {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

func setFeishuCookie(c *gin.Context, name string, value string, maxAgeSec int, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     feishuOAuthCookiePath,
		MaxAge:   maxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearFeishuCookie(c *gin.Context, name string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     feishuOAuthCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) getFeishuOAuthConfig(ctx context.Context) (service.FeishuConnectOAuthConfig, error) {
	if h != nil && h.settingSvc != nil {
		return h.settingSvc.GetFeishuConnectOAuthConfig(ctx)
	}
	if h == nil || h.cfg == nil {
		return service.FeishuConnectOAuthConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}
	if !h.cfg.Feishu.Enabled {
		return service.FeishuConnectOAuthConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "feishu oauth login is disabled")
	}
	return service.FeishuConnectOAuthConfig{
		Enabled:             h.cfg.Feishu.Enabled,
		AppID:               strings.TrimSpace(h.cfg.Feishu.AppID),
		AppSecret:           strings.TrimSpace(h.cfg.Feishu.AppSecret),
		AuthorizeURL:        strings.TrimSpace(h.cfg.Feishu.AuthorizeURL),
		TokenURL:            strings.TrimSpace(h.cfg.Feishu.TokenURL),
		UserInfoURL:         strings.TrimSpace(h.cfg.Feishu.UserInfoURL),
		Scopes:              strings.TrimSpace(h.cfg.Feishu.Scopes),
		RedirectURL:         strings.TrimSpace(h.cfg.Feishu.RedirectURL),
		FrontendRedirectURL: strings.TrimSpace(h.cfg.Feishu.FrontendRedirectURL),
	}, nil
}

func (h *AuthHandler) getFeishuOAuthConfigForIntent(ctx context.Context, intent string) (service.FeishuConnectOAuthConfig, error) {
	if intent != oauthIntentFeishuNotifyBind {
		return h.getFeishuOAuthConfig(ctx)
	}
	if h == nil || h.feishuNotificationService == nil {
		return service.FeishuConnectOAuthConfig{}, infraerrors.ServiceUnavailable("FEISHU_NOTIFICATION_NOT_READY", "feishu notification service is not ready")
	}
	notifyCfg, err := h.feishuNotificationService.GetConfig(ctx)
	if err != nil {
		return service.FeishuConnectOAuthConfig{}, err
	}
	if strings.TrimSpace(notifyCfg.AppID) == "" || strings.TrimSpace(notifyCfg.AppSecret) == "" {
		return service.FeishuConnectOAuthConfig{}, infraerrors.BadRequest("FEISHU_NOTIFICATION_APP_NOT_CONFIGURED", "feishu notification app is not configured")
	}
	base := service.FeishuConnectOAuthConfig{
		Enabled:             true,
		AuthorizeURL:        "https://accounts.feishu.cn/open-apis/authen/v1/authorize",
		TokenURL:            "https://open.feishu.cn/open-apis/authen/v2/oauth/token",
		UserInfoURL:         "https://open.feishu.cn/open-apis/authen/v1/user_info",
		FrontendRedirectURL: feishuOAuthDefaultFrontendCB,
	}
	if h.cfg != nil {
		base.AuthorizeURL = firstNonEmpty(h.cfg.Feishu.AuthorizeURL, base.AuthorizeURL)
		base.TokenURL = firstNonEmpty(h.cfg.Feishu.TokenURL, base.TokenURL)
		base.UserInfoURL = firstNonEmpty(h.cfg.Feishu.UserInfoURL, base.UserInfoURL)
		base.Scopes = strings.TrimSpace(h.cfg.Feishu.Scopes)
		base.RedirectURL = strings.TrimSpace(h.cfg.Feishu.RedirectURL)
		base.FrontendRedirectURL = firstNonEmpty(h.cfg.Feishu.FrontendRedirectURL, base.FrontendRedirectURL)
	}
	if h.settingSvc != nil {
		settings, err := h.settingSvc.GetAllSettings(ctx)
		if err != nil {
			return service.FeishuConnectOAuthConfig{}, err
		}
		base.AuthorizeURL = firstNonEmpty(settings.FeishuConnectAuthorizeURL, base.AuthorizeURL)
		base.TokenURL = firstNonEmpty(settings.FeishuConnectTokenURL, base.TokenURL)
		base.UserInfoURL = firstNonEmpty(settings.FeishuConnectUserInfoURL, base.UserInfoURL)
		base.Scopes = firstNonEmpty(settings.FeishuConnectScopes, base.Scopes)
		base.RedirectURL = firstNonEmpty(settings.FeishuConnectRedirectURL, base.RedirectURL)
		base.FrontendRedirectURL = firstNonEmpty(settings.FeishuConnectFrontendRedirectURL, base.FrontendRedirectURL)
	}
	base.AppID = strings.TrimSpace(notifyCfg.AppID)
	base.AppSecret = strings.TrimSpace(notifyCfg.AppSecret)
	if strings.TrimSpace(base.RedirectURL) == "" {
		return service.FeishuConnectOAuthConfig{}, infraerrors.BadRequest("FEISHU_NOTIFICATION_REDIRECT_NOT_CONFIGURED", "feishu oauth redirect url not configured")
	}
	return base, nil
}

// FeishuOAuthStart 启动飞书 OAuth 登录流程。
// GET /api/v1/auth/oauth/feishu/start?redirect=/dashboard
func (h *AuthHandler) FeishuOAuthStart(c *gin.Context) {
	intent := normalizeOAuthIntent(c.Query("intent"))
	cfg, err := h.getFeishuOAuthConfigForIntent(c.Request.Context(), intent)
	if err != nil {
		redirectOAuthError(c, feishuOAuthDefaultFrontendCB, "feishu_not_enabled", infraerrors.Message(err), "")
		return
	}

	state, err := oauth.GenerateState()
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_STATE_GEN_FAILED", "failed to generate oauth state").WithCause(err))
		return
	}

	redirectTo := sanitizeFrontendRedirectPath(c.Query("redirect"))
	if redirectTo == "" {
		redirectTo = feishuOAuthDefaultRedirectTo
	}

	browserSessionKey, err := generateOAuthPendingBrowserSession()
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_BROWSER_SESSION_GEN_FAILED", "failed to generate oauth browser session").WithCause(err))
		return
	}

	secureCookie := isRequestHTTPS(c)
	setFeishuCookie(c, feishuOAuthStateCookieName, encodeCookieValue(state), feishuOAuthCookieMaxAgeSec, secureCookie)
	setFeishuCookie(c, feishuOAuthRedirectCookie, encodeCookieValue(redirectTo), feishuOAuthCookieMaxAgeSec, secureCookie)
	setFeishuCookie(c, feishuOAuthIntentCookieName, encodeCookieValue(intent), feishuOAuthCookieMaxAgeSec, secureCookie)
	setOAuthPendingBrowserCookie(c, browserSessionKey, secureCookie)
	clearOAuthPendingSessionCookie(c, secureCookie)
	if intent == oauthIntentBindCurrentUser || intent == oauthIntentFeishuNotifyBind {
		bindCookieValue, err := h.buildOAuthBindUserCookieFromContext(c)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		setFeishuCookie(c, feishuOAuthBindUserCookieName, encodeCookieValue(bindCookieValue), feishuOAuthCookieMaxAgeSec, secureCookie)
	} else {
		clearFeishuCookie(c, feishuOAuthBindUserCookieName, secureCookie)
	}

	authURL, err := buildFeishuAuthorizeURL(cfg, state)
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_BUILD_URL_FAILED", "failed to build feishu authorization url").WithCause(err))
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// FeishuOAuthCallback 处理飞书 OAuth 回调。
// GET /api/v1/auth/oauth/feishu/callback?code=...&state=...
func (h *AuthHandler) FeishuOAuthCallback(c *gin.Context) {
	frontendCallback := feishuOAuthDefaultFrontendCB
	if cfg, cfgErr := h.getFeishuOAuthConfig(c.Request.Context()); cfgErr == nil {
		if v := strings.TrimSpace(cfg.FrontendRedirectURL); v != "" {
			frontendCallback = v
		}
	}

	if providerErr := strings.TrimSpace(c.Query("error")); providerErr != "" {
		redirectOAuthError(c, frontendCallback, "provider_error", providerErr, c.Query("error_description"))
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		redirectOAuthError(c, frontendCallback, "missing_params", "missing code/state", "")
		return
	}

	secureCookie := isRequestHTTPS(c)
	defer func() {
		clearFeishuCookie(c, feishuOAuthStateCookieName, secureCookie)
		clearFeishuCookie(c, feishuOAuthRedirectCookie, secureCookie)
		clearFeishuCookie(c, feishuOAuthIntentCookieName, secureCookie)
		clearFeishuCookie(c, feishuOAuthBindUserCookieName, secureCookie)
	}()

	expectedState, err := readCookieDecoded(c, feishuOAuthStateCookieName)
	if err != nil || expectedState == "" || state != expectedState {
		redirectOAuthError(c, frontendCallback, "invalid_state", "invalid oauth state", "")
		return
	}

	redirectTo, _ := readCookieDecoded(c, feishuOAuthRedirectCookie)
	redirectTo = sanitizeFrontendRedirectPath(redirectTo)
	if redirectTo == "" {
		redirectTo = feishuOAuthDefaultRedirectTo
	}

	intent, _ := readCookieDecoded(c, feishuOAuthIntentCookieName)
	intent = normalizeOAuthIntent(intent)

	browserSessionKey, _ := readOAuthPendingBrowserCookie(c)
	if strings.TrimSpace(browserSessionKey) == "" {
		redirectOAuthError(c, frontendCallback, "missing_browser_session", "missing oauth browser session", "")
		return
	}

	exchangeCfg, err := h.getFeishuOAuthConfigForIntent(c.Request.Context(), intent)
	if err != nil {
		redirectOAuthError(c, frontendCallback, "feishu_not_enabled", infraerrors.Message(err), "")
		return
	}
	if v := strings.TrimSpace(exchangeCfg.FrontendRedirectURL); v != "" {
		frontendCallback = v
	}

	tokenResp, err := feishuExchangeCode(c.Request.Context(), exchangeCfg, code)
	if err != nil {
		description := ""
		var exchangeErr *feishuTokenExchangeError
		if errors.As(err, &exchangeErr) && exchangeErr != nil {
			log.Printf(
				"[Feishu OAuth] token exchange failed: status=%d code=%q msg=%q error=%q error_description=%q body=%s",
				exchangeErr.StatusCode,
				exchangeErr.ProviderCode,
				exchangeErr.ProviderMessage,
				exchangeErr.ProviderError,
				exchangeErr.ProviderDescription,
				truncateLogValue(exchangeErr.Body, 2048),
			)
			description = exchangeErr.Error()
		} else {
			log.Printf("[Feishu OAuth] token exchange failed: %v", err)
			description = err.Error()
		}
		redirectOAuthError(c, frontendCallback, "token_exchange_failed", "failed to exchange oauth code", singleLine(description))
		return
	}

	profile, err := feishuFetchUserInfo(c.Request.Context(), exchangeCfg, tokenResp)
	if err != nil {
		log.Printf("[Feishu OAuth] userinfo fetch failed: %v", err)
		redirectOAuthError(c, frontendCallback, "userinfo_failed", "failed to fetch user info", "")
		return
	}

	email := feishuSyntheticEmail(profile.OpenID)
	identityKey := service.PendingAuthIdentityKey{
		ProviderType:    "feishu",
		ProviderKey:     "feishu",
		ProviderSubject: profile.OpenID,
	}
	upstreamClaims := map[string]any{
		"email":                  email,
		"username":               profile.Username,
		"subject":                profile.OpenID,
		"open_id":                profile.OpenID,
		"union_id":               profile.UnionID,
		"user_id":                profile.UserID,
		"suggested_display_name": profile.DisplayName,
		"suggested_avatar_url":   profile.AvatarURL,
	}
	if compatEmail := strings.TrimSpace(profile.Email); compatEmail != "" {
		upstreamClaims["compat_email"] = compatEmail
	}

	if intent == oauthIntentFeishuNotifyBind {
		targetUserID, err := h.readOAuthBindUserIDFromCookie(c, feishuOAuthBindUserCookieName)
		if err != nil {
			redirectOAuthError(c, frontendCallback, "invalid_state", "invalid oauth bind target", "")
			return
		}
		if err := h.bindFeishuNotificationIdentity(c.Request.Context(), targetUserID, exchangeCfg, profile); err != nil {
			redirectOAuthError(c, frontendCallback, "bind_failed", infraerrors.Reason(err), infraerrors.Message(err))
			return
		}
		c.Redirect(http.StatusFound, redirectTo)
		return
	}

	if intent == oauthIntentBindCurrentUser {
		targetUserID, err := h.readOAuthBindUserIDFromCookie(c, feishuOAuthBindUserCookieName)
		if err != nil {
			redirectOAuthError(c, frontendCallback, "invalid_state", "invalid oauth bind target", "")
			return
		}
		if err := h.createOAuthPendingSession(c, oauthPendingSessionPayload{
			Intent:                 oauthIntentBindCurrentUser,
			Identity:               identityKey,
			TargetUserID:           &targetUserID,
			ResolvedEmail:          email,
			RedirectTo:             redirectTo,
			BrowserSessionKey:      browserSessionKey,
			UpstreamIdentityClaims: upstreamClaims,
			CompletionResponse: map[string]any{
				"redirect": redirectTo,
			},
		}); err != nil {
			redirectOAuthError(c, frontendCallback, "session_error", infraerrors.Reason(err), infraerrors.Message(err))
			return
		}
		redirectToFrontendCallback(c, frontendCallback)
		return
	}

	existingIdentityUser, err := h.findOAuthIdentityUser(c.Request.Context(), identityKey)
	if err != nil {
		redirectOAuthError(c, frontendCallback, "session_error", infraerrors.Reason(err), infraerrors.Message(err))
		return
	}
	if existingIdentityUser != nil {
		if err := h.createOAuthPendingSession(c, oauthPendingSessionPayload{
			Intent:                 oauthIntentLogin,
			Identity:               identityKey,
			TargetUserID:           &existingIdentityUser.ID,
			ResolvedEmail:          existingIdentityUser.Email,
			RedirectTo:             redirectTo,
			BrowserSessionKey:      browserSessionKey,
			UpstreamIdentityClaims: upstreamClaims,
			CompletionResponse: map[string]any{
				"redirect": redirectTo,
			},
		}); err != nil {
			redirectOAuthError(c, frontendCallback, "session_error", infraerrors.Reason(err), infraerrors.Message(err))
			return
		}
		redirectToFrontendCallback(c, frontendCallback)
		return
	}

	if err := h.createLinuxDoOAuthChoicePendingSession(
		c,
		identityKey,
		email,
		email,
		redirectTo,
		browserSessionKey,
		upstreamClaims,
		strings.TrimSpace(profile.Email),
		nil,
		false,
		h.isForceEmailOnThirdPartySignup(c.Request.Context()),
	); err != nil {
		redirectOAuthError(c, frontendCallback, "session_error", "failed to continue oauth login", "")
		return
	}
	redirectToFrontendCallback(c, frontendCallback)
}

func buildFeishuAuthorizeURL(cfg service.FeishuConnectOAuthConfig, state string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(cfg.AuthorizeURL))
	if err != nil {
		return "", fmt.Errorf("parse authorize_url: %w", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", strings.TrimSpace(cfg.AppID))
	q.Set("redirect_uri", strings.TrimSpace(cfg.RedirectURL))
	if strings.TrimSpace(cfg.Scopes) != "" {
		q.Set("scope", strings.TrimSpace(cfg.Scopes))
	}
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func feishuExchangeCode(ctx context.Context, cfg service.FeishuConnectOAuthConfig, code string) (*feishuTokenResponse, error) {
	client := req.C().SetTimeout(30 * time.Second)
	bodyBytes, err := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     strings.TrimSpace(cfg.AppID),
		"client_secret": strings.TrimSpace(cfg.AppSecret),
		"code":          strings.TrimSpace(code),
		"redirect_uri":  strings.TrimSpace(cfg.RedirectURL),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal token request: %w", err)
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(bodyBytes).
		Post(strings.TrimSpace(cfg.TokenURL))
	if err != nil {
		return nil, fmt.Errorf("request token: %w", err)
	}

	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() || feishuAPIErrorCode(body) != 0 {
		providerErr, providerDesc := parseOAuthProviderError(body)
		return nil, &feishuTokenExchangeError{
			StatusCode:          resp.StatusCode,
			ProviderCode:        strings.TrimSpace(getGJSON(body, "code")),
			ProviderMessage:     firstNonEmpty(getGJSON(body, "msg"), getGJSON(body, "message")),
			ProviderError:       providerErr,
			ProviderDescription: providerDesc,
			Body:                body,
		}
	}

	tokenResp := &feishuTokenResponse{
		AccessToken:  firstNonEmpty(getGJSON(body, "access_token"), getGJSON(body, "data.access_token")),
		TokenType:    firstNonEmpty(getGJSON(body, "token_type"), getGJSON(body, "data.token_type")),
		ExpiresIn:    firstNonZeroInt64(gjson.Get(body, "expires_in").Int(), gjson.Get(body, "data.expires_in").Int()),
		RefreshToken: firstNonEmpty(getGJSON(body, "refresh_token"), getGJSON(body, "data.refresh_token")),
		Scope:        firstNonEmpty(getGJSON(body, "scope"), getGJSON(body, "data.scope")),
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return nil, &feishuTokenExchangeError{StatusCode: resp.StatusCode, Body: body}
	}
	if strings.TrimSpace(tokenResp.TokenType) == "" {
		tokenResp.TokenType = "Bearer"
	}
	return tokenResp, nil
}

type feishuUserProfile struct {
	OpenID      string
	UnionID     string
	UserID      string
	TenantKey   string
	Email       string
	Username    string
	DisplayName string
	AvatarURL   string
}

func feishuFetchUserInfo(ctx context.Context, cfg service.FeishuConnectOAuthConfig, token *feishuTokenResponse) (feishuUserProfile, error) {
	authorization, err := buildBearerAuthorization(token.TokenType, token.AccessToken)
	if err != nil {
		return feishuUserProfile{}, fmt.Errorf("invalid token for userinfo request: %w", err)
	}

	client := req.C().SetTimeout(30 * time.Second)
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", authorization).
		Get(strings.TrimSpace(cfg.UserInfoURL))
	if err != nil {
		return feishuUserProfile{}, fmt.Errorf("request userinfo: %w", err)
	}
	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() || feishuAPIErrorCode(body) != 0 {
		return feishuUserProfile{}, fmt.Errorf("userinfo status=%d code=%s msg=%s", resp.StatusCode, getGJSON(body, "code"), firstNonEmpty(getGJSON(body, "msg"), getGJSON(body, "message")))
	}
	return feishuParseUserInfo(body)
}

func feishuParseUserInfo(body string) (feishuUserProfile, error) {
	openID := firstNonEmpty(
		getGJSON(body, "open_id"),
		getGJSON(body, "data.open_id"),
		getGJSON(body, "user.open_id"),
	)
	if !isSafeFeishuSubject(openID) {
		return feishuUserProfile{}, errors.New("userinfo missing or invalid open_id field")
	}

	unionID := firstNonEmpty(getGJSON(body, "union_id"), getGJSON(body, "data.union_id"), getGJSON(body, "user.union_id"))
	userID := firstNonEmpty(getGJSON(body, "user_id"), getGJSON(body, "data.user_id"), getGJSON(body, "user.user_id"))
	tenantKey := firstNonEmpty(getGJSON(body, "tenant_key"), getGJSON(body, "data.tenant_key"), getGJSON(body, "user.tenant_key"))
	displayName := firstNonEmpty(
		getGJSON(body, "name"),
		getGJSON(body, "data.name"),
		getGJSON(body, "user.name"),
		getGJSON(body, "en_name"),
		getGJSON(body, "data.en_name"),
		openID,
	)
	email := firstNonEmpty(getGJSON(body, "email"), getGJSON(body, "data.email"), getGJSON(body, "user.email"))
	avatarURL := firstNonEmpty(
		getGJSON(body, "avatar_url"),
		getGJSON(body, "data.avatar_url"),
		getGJSON(body, "avatar_thumb"),
		getGJSON(body, "data.avatar_thumb"),
		getGJSON(body, "avatar_middle"),
		getGJSON(body, "data.avatar_middle"),
		getGJSON(body, "avatar_big"),
		getGJSON(body, "data.avatar_big"),
	)
	username := normalizeFeishuUsername(displayName, openID)
	return feishuUserProfile{
		OpenID:      strings.TrimSpace(openID),
		UnionID:     strings.TrimSpace(unionID),
		UserID:      strings.TrimSpace(userID),
		TenantKey:   strings.TrimSpace(tenantKey),
		Email:       strings.TrimSpace(email),
		Username:    username,
		DisplayName: strings.TrimSpace(displayName),
		AvatarURL:   strings.TrimSpace(avatarURL),
	}, nil
}

func (h *AuthHandler) bindFeishuNotificationIdentity(ctx context.Context, userID int64, oauthCfg service.FeishuConnectOAuthConfig, profile feishuUserProfile) error {
	if h == nil || h.feishuNotificationService == nil {
		return infraerrors.ServiceUnavailable("FEISHU_NOTIFICATION_NOT_READY", "feishu notification service is not ready")
	}
	cfg, err := h.feishuNotificationService.GetConfig(ctx)
	if err != nil {
		return err
	}
	appID := firstNonEmpty(cfg.AppID, oauthCfg.AppID)
	if appID == "" {
		return infraerrors.BadRequest("FEISHU_NOTIFICATION_APP_NOT_CONFIGURED", "feishu notification app id not configured")
	}
	_, err = h.feishuNotificationService.UpsertNotifyBinding(ctx, service.UpsertFeishuUserIdentityBindingInput{
		UserID:    userID,
		AppID:     appID,
		TenantKey: profile.TenantKey,
		OpenID:    profile.OpenID,
		UnionID:   profile.UnionID,
		Metadata: map[string]any{
			"oauth_app_id": strings.TrimSpace(oauthCfg.AppID),
			"provider":     "feishu",
			"user_id":      profile.UserID,
			"display_name": profile.DisplayName,
			"avatar_url":   profile.AvatarURL,
			"compat_email": profile.Email,
			"bound_via":    "oauth",
		},
	})
	return err
}

func feishuAPIErrorCode(body string) int64 {
	body = strings.TrimSpace(body)
	if body == "" {
		return 0
	}
	code := gjson.Get(body, "code")
	if !code.Exists() {
		return 0
	}
	return code.Int()
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func isSafeFeishuSubject(subject string) bool {
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > feishuOAuthMaxSubjectLen {
		return false
	}
	for _, r := range subject {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func normalizeFeishuUsername(displayName string, openID string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName != "" {
		return displayName
	}
	return "feishu_" + strings.TrimSpace(openID)
}

func feishuSyntheticEmail(openID string) string {
	openID = strings.ToLower(strings.TrimSpace(openID))
	if openID == "" {
		return ""
	}
	return "feishu-" + openID + service.FeishuConnectSyntheticEmailDomain
}

type completeFeishuOAuthRequest struct {
	InvitationCode   string `json:"invitation_code" binding:"required"`
	AffCode          string `json:"aff_code,omitempty"`
	AdoptDisplayName *bool  `json:"adopt_display_name,omitempty"`
	AdoptAvatar      *bool  `json:"adopt_avatar,omitempty"`
}

// CompleteFeishuOAuthRegistration completes a pending Feishu OAuth registration.
// POST /api/v1/auth/oauth/feishu/complete-registration
func (h *AuthHandler) CompleteFeishuOAuthRegistration(c *gin.Context) {
	var req completeFeishuOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	secureCookie := isRequestHTTPS(c)
	sessionToken, err := readOAuthPendingSessionCookie(c)
	if err != nil {
		clearOAuthPendingSessionCookie(c, secureCookie)
		clearOAuthPendingBrowserCookie(c, secureCookie)
		response.ErrorFrom(c, service.ErrPendingAuthSessionNotFound)
		return
	}
	browserSessionKey, err := readOAuthPendingBrowserCookie(c)
	if err != nil {
		clearOAuthPendingSessionCookie(c, secureCookie)
		clearOAuthPendingBrowserCookie(c, secureCookie)
		response.ErrorFrom(c, service.ErrPendingAuthBrowserMismatch)
		return
	}
	pendingSvc, err := h.pendingIdentityService()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	session, err := pendingSvc.GetBrowserSession(c.Request.Context(), sessionToken, browserSessionKey)
	if err != nil {
		clearOAuthPendingSessionCookie(c, secureCookie)
		clearOAuthPendingBrowserCookie(c, secureCookie)
		response.ErrorFrom(c, err)
		return
	}
	if err := ensurePendingOAuthCompleteRegistrationSession(session); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if updatedSession, handled, err := h.legacyCompleteRegistrationSessionStatus(c, session); err != nil {
		response.ErrorFrom(c, err)
		return
	} else if handled {
		c.JSON(http.StatusOK, buildPendingOAuthSessionStatusPayload(updatedSession))
		return
	} else {
		session = updatedSession
	}
	if err := h.ensureBackendModeAllowsNewUserLogin(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	email := strings.TrimSpace(session.ResolvedEmail)
	username := pendingSessionStringValue(session.UpstreamIdentityClaims, "username")
	if username == "" {
		username = pendingSessionStringValue(session.UpstreamIdentityClaims, "subject")
	}
	if email == "" || username == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("PENDING_AUTH_SESSION_INVALID", "pending auth registration context is invalid"))
		return
	}

	client := h.entClient()
	if client == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("PENDING_AUTH_NOT_READY", "pending auth service is not ready"))
		return
	}
	if err := ensurePendingOAuthRegistrationIdentityAvailable(c.Request.Context(), client, session); err != nil {
		respondPendingOAuthBindingApplyError(c, err)
		return
	}
	decision, err := h.ensurePendingOAuthAdoptionDecision(c, session.ID, oauthAdoptionDecisionRequest{
		AdoptDisplayName: req.AdoptDisplayName,
		AdoptAvatar:      req.AdoptAvatar,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	tokenPair, user, err := h.authService.LoginOrRegisterOAuthWithTokenPair(c.Request.Context(), email, username, req.InvitationCode, req.AffCode, "feishu")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := applyPendingOAuthAdoptionAndConsumeSession(c.Request.Context(), client, h.authService, h.userService, session, decision, user.ID); err != nil {
		respondPendingOAuthBindingApplyError(c, err)
		return
	}
	h.authService.RecordSuccessfulLogin(c.Request.Context(), user.ID)
	clearOAuthPendingSessionCookie(c, secureCookie)
	clearOAuthPendingBrowserCookie(c, secureCookie)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

func (h *AuthHandler) CreateFeishuOAuthAccount(c *gin.Context) {
	h.createPendingOAuthAccount(c, "feishu")
}

func (h *AuthHandler) BindFeishuOAuthLogin(c *gin.Context) {
	h.bindPendingOAuthLogin(c, "feishu")
}
