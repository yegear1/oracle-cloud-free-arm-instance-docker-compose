package notify_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"oracle-fisher/internal/notify"
)

func TestSendWithMockServer(t *testing.T) {
	var receivedBody []byte
	var receivedHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom-Header")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	os.Setenv("NOTIFICATION_WEBHOOK_URL", server.URL)
	os.Setenv("NOTIFICATION_WEBHOOK_METHOD", "POST")
	os.Setenv("NOTIFICATION_WEBHOOK_HEADERS", `{"X-Custom-Header": "unit-test-val"}`)
	os.Setenv("NOTIFICATION_WEBHOOK_BODY", `{"content": "{{message}}"}`)
	defer func() {
		os.Unsetenv("NOTIFICATION_WEBHOOK_URL")
		os.Unsetenv("NOTIFICATION_WEBHOOK_METHOD")
		os.Unsetenv("NOTIFICATION_WEBHOOK_HEADERS")
		os.Unsetenv("NOTIFICATION_WEBHOOK_BODY")
	}()

	notify.Send("Instance Created Successfully!")

	if receivedHeader != "unit-test-val" {
		t.Errorf("expected header 'unit-test-val', got '%s'", receivedHeader)
	}

	var parsed map[string]string
	if err := json.Unmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("failed to parse received JSON body: %v", err)
	}

	if parsed["content"] != "Instance Created Successfully!" {
		t.Errorf("expected content 'Instance Created Successfully!', got '%s'", parsed["content"])
	}
}
