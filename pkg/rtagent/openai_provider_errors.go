package rtagent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func parseOpenAIProviderError(statusCode int, body []byte) error {
	var decoded struct {
		Error struct {
			Code    any    `json:"code,omitempty"`
			Type    string `json:"type,omitempty"`
			Message string `json:"message,omitempty"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &decoded)
	code := strings.TrimSpace(fmt.Sprint(decoded.Error.Code))
	if code == "<nil>" {
		code = ""
	}
	message := strings.TrimSpace(decoded.Error.Message)
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	return &openAICompatibleProviderError{
		Provider:     "openai-compatible",
		StatusCode:   statusCode,
		Code:         firstNonEmpty(code, decoded.Error.Type),
		Message:      message,
		Retryable:    statusCode == http.StatusTooManyRequests || statusCode >= 500,
		RateLimited:  statusCode == http.StatusTooManyRequests,
		SafeForModel: statusCode >= 400 && statusCode < 500 && statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden,
		BodyPreview:  previewString(string(body), 4096),
		Body:         previewString(string(body), 4096),
	}
}
