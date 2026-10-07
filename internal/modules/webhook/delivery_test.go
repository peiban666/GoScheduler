package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManualSendSignsAndChecksResponse(t *testing.T) {
	for _, provider := range []string{DingTalk, Feishu} {
		t.Run(provider, func(t *testing.T) {
			count := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				body, _ := io.ReadAll(r.Body)
				if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
					t.Error("bad test request")
				}
				if !strings.Contains(string(body), "Webhook 测试") {
					t.Error("missing identifiable test message")
				}
				if provider == DingTalk {
					if r.URL.Query().Get("sign") == "" || r.URL.Query().Get("timestamp") == "" {
						t.Error("missing DingTalk signature")
					}
					w.Write([]byte(`{"errcode":0}`))
				} else {
					var object map[string]interface{}
					json.Unmarshal(body, &object)
					if object["sign"] == nil || object["timestamp"] == nil {
						t.Error("missing Feishu signature")
					}
					w.Write([]byte(`{"code":0}`))
				}
			}))
			defer server.Close()
			template := `{"msgtype":"text","text":{"content":"{{.TaskName}} {{.Result}}"}}`
			if provider == Feishu {
				template = `{"msg_type":"text","content":{"text":"{{.TaskName}} {{.Result}}"}}`
			}
			err := TestSend(Config{Url: server.URL, Template: template, Provider: provider, SignEnabled: true, Secret: "fixture-secret"})
			if err != nil || count != 1 {
				t.Fatalf("test send failed: %v, requests: %d", err, count)
			}
		})
	}
}

func TestPostNeverFollowsRedirectOrLeaksResponse(t *testing.T) {
	received := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received++ }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	status, body := Post(redirect.URL+"?access_token=fixture-token", `{}`, 1)
	if status != 307 || received != 0 {
		t.Fatal("credentials were forwarded to redirected endpoint")
	}
	err := CheckResponse(Generic, status, body)
	if err == nil || strings.Contains(err.Error(), target.URL) {
		t.Fatal("unsafe or missing redirect error")
	}
}
