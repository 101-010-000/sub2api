package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type contentModerationNormalizedContext struct {
	Protocol string         `json:"protocol"`
	UserText string         `json:"user_text"`
	Images   []string       `json:"images,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

func buildContentModerationNormalizedContext(protocol string, body []byte, input ContentModerationCheckInput) (*contentModerationNormalizedContext, []byte, string, error) {
	if len(body) == 0 {
		return nil, nil, "", nil
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, nil, "", err
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return nil, nil, "", nil
	}
	clean, ok := sanitizeModerationContextValue(obj).(map[string]any)
	if !ok {
		return nil, nil, "", nil
	}
	ctx := &contentModerationNormalizedContext{
		Protocol: strings.TrimSpace(protocol),
		Meta: map[string]any{
			"request_id": input.RequestID,
			"endpoint":   input.Endpoint,
			"provider":   input.Provider,
			"model":      input.Model,
		},
	}
	userInput := extractUserFocusedModerationInputFromSanitizedContext(protocol, clean)
	ctx.UserText = userInput.Text
	ctx.Images = userInput.Images
	raw, err := json.Marshal(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	sum := sha256.Sum256(raw)
	return ctx, raw, hex.EncodeToString(sum[:]), nil
}

func contentModerationContextSummary(ctx *contentModerationNormalizedContext) string {
	if ctx == nil {
		return ""
	}
	if strings.TrimSpace(ctx.UserText) != "" {
		return trimRunes(redactContentModerationSecrets(ctx.UserText), maxModerationExcerptRunes)
	}
	return ""
}

func parseContentModerationNormalizedContext(raw string) (*contentModerationNormalizedContext, ContentModerationInput, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ContentModerationInput{}, nil
	}
	var ctx contentModerationNormalizedContext
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		return nil, ContentModerationInput{}, err
	}
	input := ContentModerationInput{
		Text:   trimRunesFromEnd(normalizeContentModerationTextPreserveLines(ctx.UserText), maxModerationInputRunes),
		Images: normalizeModerationImages(ctx.Images),
	}
	return &ctx, input, nil
}

func (s *ContentModerationService) captureModerationContext(ctx context.Context, cfg *ContentModerationConfig, input ContentModerationCheckInput, inputHash string) *int64 {
	if s == nil || s.repo == nil || cfg == nil || !cfg.ContextCaptureEnabled {
		return nil
	}
	if s.encryptor == nil {
		s.recordContextCaptureError("context capture disabled: encryptor unavailable")
		return nil
	}
	normalized, raw, contextHash, err := buildContentModerationNormalizedContext(input.Protocol, input.Body, input)
	if err != nil {
		s.recordContextCaptureError(err.Error())
		return nil
	}
	if normalized == nil || strings.TrimSpace(normalized.UserText) == "" && len(normalized.Images) == 0 {
		return nil
	}
	if len(raw) > cfg.ContextMaxBytes {
		s.contextDrops.Add(1)
		s.recordContextCaptureError(fmt.Sprintf("context too large: %d bytes", len(raw)))
		return nil
	}
	encrypted, err := s.encryptor.Encrypt(string(raw))
	if err != nil {
		s.contextDrops.Add(1)
		s.recordContextCaptureError(err.Error())
		return nil
	}
	now := time.Now()
	item := &ContentModerationContext{
		RequestID:         input.RequestID,
		UserID:            positiveInt64Ptr(input.UserID),
		UserEmail:         input.UserEmail,
		APIKeyID:          positiveInt64Ptr(input.APIKeyID),
		APIKeyName:        input.APIKeyName,
		GroupID:           cloneInt64Ptr(input.GroupID),
		GroupName:         input.GroupName,
		Endpoint:          input.Endpoint,
		Provider:          input.Provider,
		Model:             input.Model,
		Protocol:          input.Protocol,
		InputHash:         inputHash,
		ContextHash:       contextHash,
		EncryptedContext:  encrypted,
		ContextSummary:    contentModerationContextSummary(normalized),
		ContextBytes:      len(raw),
		Status:            ContentModerationContextStatusPending,
		ReviewStage:       ContentModerationReviewStageBackground,
		MaxReviewAttempts: cfg.BackgroundReviewMaxAttempts,
		NextReviewAt:      now,
	}
	if err := s.repo.CreateContext(ctx, item); err != nil {
		s.contextDrops.Add(1)
		s.recordContextCaptureError(err.Error())
		return nil
	}
	s.clearContextCaptureError()
	return &item.ID
}

func (s *ContentModerationService) decryptModerationContext(ctx context.Context, item *ContentModerationContext, adminUserID int64) (*ContentModerationContext, error) {
	if item == nil {
		return nil, nil
	}
	if s == nil || s.encryptor == nil {
		return nil, infraerrors.InternalServer("CONTENT_MODERATION_CONTEXT_DECRYPT_UNAVAILABLE", "内容审计上下文解密器不可用")
	}
	plain, err := s.encryptor.Decrypt(item.EncryptedContext)
	if err != nil {
		return nil, fmt.Errorf("decrypt content moderation context: %w", err)
	}
	out := *item
	out.PlainContext = plain
	if s.repo != nil {
		_ = s.repo.CreateContextAccessLog(ctx, item.ID, adminUserID, "view")
	}
	return &out, nil
}

func (s *ContentModerationService) recordContextCaptureError(message string) {
	if s == nil {
		return
	}
	s.contextErrorMu.Lock()
	defer s.contextErrorMu.Unlock()
	s.contextCaptureError = trimRunes(strings.TrimSpace(message), 500)
	s.lastContextErrorAt = time.Now()
}

func (s *ContentModerationService) clearContextCaptureError() {
	if s == nil {
		return
	}
	s.contextErrorMu.Lock()
	defer s.contextErrorMu.Unlock()
	s.contextCaptureError = ""
}

func (s *ContentModerationService) contextCaptureErrorSnapshot() (string, *time.Time) {
	if s == nil {
		return "", nil
	}
	s.contextErrorMu.Lock()
	defer s.contextErrorMu.Unlock()
	if s.lastContextErrorAt.IsZero() {
		return s.contextCaptureError, nil
	}
	t := s.lastContextErrorAt
	return s.contextCaptureError, &t
}

func positiveInt64Ptr(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func sanitizeModerationContextValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveModerationContextKey(key) {
				out[key] = "[已脱敏]"
				continue
			}
			out[key] = sanitizeModerationContextValue(child)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, sanitizeModerationContextValue(child))
		}
		return out
	default:
		return value
	}
}

func isSensitiveModerationContextKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	for _, part := range []string{"authorization", "api_key", "apikey", "access_token", "refresh_token", "id_token", "session_token", "cookie", "password", "passwd", "secret", "private_key"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return key == "headers" || key == "header"
}

func extractUserFocusedModerationInputFromSanitizedContext(protocol string, root map[string]any) ContentModerationInput {
	var parts []string
	var images []string
	switch protocol {
	case ContentModerationProtocolAnthropicMessages, ContentModerationProtocolOpenAIChat:
		collectUserMessagesFromAny(root["messages"], &parts, &images)
	case ContentModerationProtocolOpenAIResponses:
		collectResponsesUserInputsFromAny(root["input"], &parts, &images)
	case ContentModerationProtocolGemini:
		collectGeminiUserContentsFromAny(root["contents"], &parts, &images)
	case ContentModerationProtocolOpenAIImages:
		collectContextValue(root["prompt"], &parts, &images)
	default:
		collectUserMessagesFromAny(root["messages"], &parts, &images)
		collectResponsesUserInputsFromAny(root["input"], &parts, &images)
		collectGeminiUserContentsFromAny(root["contents"], &parts, &images)
		collectContextValue(root["prompt"], &parts, &images)
	}
	out := ContentModerationInput{
		Text:   trimRunesFromEnd(normalizeContentModerationTextPreserveLines(strings.Join(parts, "\n")), maxModerationInputRunes),
		Images: normalizeModerationImages(images),
	}
	return out
}

func collectUserMessagesFromAny(value any, parts *[]string, images *[]string) {
	items, ok := value.([]any)
	if !ok {
		return
	}
	if !lastAnyRoleIs(items, "user") {
		return
	}
	if !anyUserMessageHasModerationInput(items[len(items)-1]) {
		return
	}
	start := recentAnyRoleMessageStart(items, "user")
	for _, item := range items[start:] {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["role"])))
		if role != "user" {
			continue
		}
		collectContextValue(obj["content"], parts, images)
	}
}

func collectResponsesUserInputsFromAny(value any, parts *[]string, images *[]string) {
	switch v := value.(type) {
	case string:
		addModerationText(parts, v)
	case []any:
		if len(v) > 0 && !isResponsesUserContextItem(v[len(v)-1]) {
			return
		}
		start := recentResponsesContextInputStart(v)
		for _, item := range v[start:] {
			if isResponsesUserContextItem(item) {
				collectResponsesItemValue(item, parts, images)
			}
		}
	case map[string]any:
		if isResponsesUserContextItem(v) {
			collectResponsesItemValue(v, parts, images)
		}
	}
}

func isResponsesUserContextItem(value any) bool {
	obj, ok := value.(map[string]any)
	if !ok {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["role"])))
	if role == "user" {
		return true
	}
	return role == "" && (obj["type"] == "input_text" || obj["text"] != nil || obj["content"] != nil)
}

func collectResponsesItemValue(value any, parts *[]string, images *[]string) {
	obj, ok := value.(map[string]any)
	if !ok {
		return
	}
	collectContextValue(obj["content"], parts, images)
	if obj["type"] == "input_text" || obj["text"] != nil {
		collectContextValue(obj, parts, images)
	}
}

func collectGeminiUserContentsFromAny(value any, parts *[]string, images *[]string) {
	items, ok := value.([]any)
	if !ok {
		return
	}
	if !lastAnyGeminiRoleIsUser(items) {
		return
	}
	if !anyGeminiContentHasModerationInput(items[len(items)-1]) {
		return
	}
	start := recentAnyGeminiUserContentStart(items)
	for _, item := range items[start:] {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["role"])))
		if role != "" && role != "user" {
			continue
		}
		collectContextValue(obj["parts"], parts, images)
	}
}

func lastAnyRoleIs(items []any, role string) bool {
	if len(items) == 0 {
		return false
	}
	obj, ok := items[len(items)-1].(map[string]any)
	if !ok {
		return false
	}
	return strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["role"]))) == role
}

func anyUserMessageHasModerationInput(item any) bool {
	obj, ok := item.(map[string]any)
	if !ok {
		return false
	}
	var parts []string
	var images []string
	collectContextValue(obj["content"], &parts, &images)
	return normalizeContentModerationText(strings.Join(parts, "\n")) != "" || len(images) > 0
}

func recentAnyRoleMessageStart(items []any, role string) int {
	seen := 0
	for i := len(items) - 1; i >= 0; i-- {
		obj, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["role"]))) != role {
			continue
		}
		seen++
		if seen >= maxModerationUserMessageWindow {
			return i
		}
	}
	return 0
}

func recentResponsesContextInputStart(items []any) int {
	seen := 0
	for i := len(items) - 1; i >= 0; i-- {
		if !isResponsesUserContextItem(items[i]) {
			continue
		}
		seen++
		if seen >= maxModerationUserMessageWindow {
			return i
		}
	}
	return 0
}

func lastAnyGeminiRoleIsUser(items []any) bool {
	if len(items) == 0 {
		return false
	}
	obj, ok := items[len(items)-1].(map[string]any)
	if !ok {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["role"])))
	return role == "" || role == "user"
}

func anyGeminiContentHasModerationInput(item any) bool {
	obj, ok := item.(map[string]any)
	if !ok {
		return false
	}
	var parts []string
	var images []string
	collectContextValue(obj["parts"], &parts, &images)
	return normalizeContentModerationText(strings.Join(parts, "\n")) != "" || len(images) > 0
}

func recentAnyGeminiUserContentStart(items []any) int {
	seen := 0
	for i := len(items) - 1; i >= 0; i-- {
		obj, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["role"])))
		if role != "" && role != "user" {
			continue
		}
		seen++
		if seen >= maxModerationUserMessageWindow {
			return i
		}
	}
	return 0
}

func collectContextValue(value any, parts *[]string, images *[]string) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		addModerationText(parts, v)
	case []any:
		for _, item := range v {
			collectContextValue(item, parts, images)
		}
	case map[string]any:
		addModerationImage(images, contextString(v["image_url.url"]))
		addModerationImage(images, contextString(v["image_url"]))
		addModerationImage(images, contextString(v["url"]))
		addModerationImageData(images, contextString(v["media_type"]), contextString(v["data"]))
		addModerationImageData(images, contextString(v["mime_type"]), contextString(v["data"]))
		addModerationImage(images, contextString(v["data"]))
		addModerationImage(images, contextString(v["base64"]))
		if text := contextString(v["text"]); text != "" {
			addModerationText(parts, text)
		}
		if content, ok := v["content"]; ok {
			collectContextValue(content, parts, images)
		}
		if partsValue, ok := v["parts"]; ok {
			collectContextValue(partsValue, parts, images)
		}
	}
}

func contextString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		if url, ok := v["url"].(string); ok {
			return strings.TrimSpace(url)
		}
	}
	return ""
}

func contextValueText(value any) string {
	var parts []string
	var images []string
	collectContextValue(value, &parts, &images)
	return normalizeContentModerationText(strings.Join(parts, "\n"))
}
