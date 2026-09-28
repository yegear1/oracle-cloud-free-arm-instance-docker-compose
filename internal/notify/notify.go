package notify

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"oracle-fisher/internal/config"
	"oracle-fisher/internal/logging"
)

func unescapeQuotes(s string) string {
	return strings.ReplaceAll(s, `\"`, `"`)
}

// Send dispara notificações via webhook HTTP (WhatsApp API, Discord, Slack, etc.).
func Send(msg string) {
	webhookURL := config.Get("NOTIFICATION_WEBHOOK_URL", "")
	if webhookURL == "" {
		return
	}

	slog.Info("Enviando notificação via webhook")
	method := strings.ToUpper(config.Get("NOTIFICATION_WEBHOOK_METHOD", "POST"))
	headersStr := unescapeQuotes(config.Get("NOTIFICATION_WEBHOOK_HEADERS", ""))
	bodyTemplate := unescapeQuotes(config.Get("NOTIFICATION_WEBHOOK_BODY", ""))

	var bodyBytes []byte
	if bodyTemplate != "" {
		escapedMsg := strings.ReplaceAll(msg, "\n", "\\n")
		escapedMsg = strings.ReplaceAll(escapedMsg, "\"", "\\\"")
		bodyStr := strings.ReplaceAll(bodyTemplate, "{{message}}", escapedMsg)
		bodyStr = strings.ReplaceAll(bodyStr, "{message}", escapedMsg)
		bodyBytes = []byte(bodyStr)
	} else {
		payload := map[string]string{
			"message": msg,
			"content": msg,
			"text":    msg,
		}
		bodyBytes, _ = json.Marshal(payload)
	}

	req, err := http.NewRequest(method, webhookURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		logging.Error("Erro ao instanciar requisição de webhook", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if headersStr != "" {
		var headerMap map[string]string
		if err := json.Unmarshal([]byte(headersStr), &headerMap); err == nil {
			for k, v := range headerMap {
				req.Header.Set(k, v)
			}
		} else {
			for _, h := range strings.Split(headersStr, ";") {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logging.Error("Falha ao enviar notificação via webhook", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("Notificação disparada com sucesso", "status_code", resp.StatusCode)
	} else {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		slog.Warn("Webhook retornou status HTTP de erro",
			"status_code", resp.StatusCode,
			"response_snippet", strings.TrimSpace(string(bodySnippet)),
		)
	}
}
