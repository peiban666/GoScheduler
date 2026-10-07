package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gaggad/goscheduler/internal/models"
	"github.com/gaggad/goscheduler/internal/modules/httpclient"
	"github.com/gaggad/goscheduler/internal/modules/webhook"
)

func TestWebhookRetriesBusinessFailureWithFreshSignatures(t *testing.T) {
	var timestamps []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("wrong HTTP method or content type")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"msgtype":"text"`) {
			t.Error("wrong robot body")
		}
		timestamps = append(timestamps, r.URL.Query().Get("timestamp"))
		if r.URL.Query().Get("sign") == "" || r.URL.Query().Get("access_token") != "fixture-token" {
			t.Error("signature or existing query token missing")
		}
		if len(timestamps) == 1 {
			w.Write([]byte(`{"errcode":310000}`))
		} else {
			w.Write([]byte(`{"errcode":0}`))
		}
	}))
	defer server.Close()
	sleeps := 0
	tick := time.Unix(1599360473, 0)
	config := models.WebHook{Url: server.URL + "?access_token=fixture-token",
		Provider: webhook.DingTalk, SignEnabled: true, Secret: "fixture-secret"}
	err := sendWebhook(config, `{"msgtype":"text","text":{"content":"hello"}}`,
		httpclient.PostJson, func() time.Time { tick = tick.Add(time.Second); return tick },
		func(time.Duration) { sleeps++ })
	if err != nil || sleeps != 1 || len(timestamps) != 2 || timestamps[0] == timestamps[1] {
		t.Fatalf("retry failed: %v; %v", err, timestamps)
	}
}

func TestWebhookFailureNeverIncludesTokenSecretOrServerBody(t *testing.T) {
	attempts, sleeps := 0, 0
	config := models.WebHook{Url: "https://example.invalid/robot?access_token=fixture-token",
		Provider: webhook.Feishu, SignEnabled: true, Secret: "fixture-secret"}
	err := sendWebhook(config, `{"msg_type":"text","content":{"text":"hello"}}`,
		func(address, body string, timeout int) httpclient.ResponseWrapper {
			attempts++
			if _, err := url.Parse(address); err != nil {
				t.Fatal(err)
			}
			return httpclient.ResponseWrapper{StatusCode: 200, Body: `{"code":19021,"msg":"fixture-secret fixture-token"}`}
		}, time.Now, func(time.Duration) { sleeps++ })
	if err == nil || attempts != 3 || sleeps != 2 ||
		strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), "fixture-token") {
		t.Fatalf("unsafe retry error: %v", err)
	}
}
