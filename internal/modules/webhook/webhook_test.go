package webhook

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

var fixedTime = time.Unix(1599360473, 123000000)

func TestProviderSignatureVectors(t *testing.T) {
	body := `{"msgtype":"text","text":{"content":"hello"}}`
	config := Config{Url: "https://example.invalid/robot?access_token=fixture%2Btoken&timestamp=old&sign=old",
		Provider: DingTalk, SignEnabled: true, Secret: "fixture-secret"}
	address, sent, err := Request(config, body, fixedTime)
	if err != nil || sent != body {
		t.Fatalf("request failed: %v", err)
	}
	query, _ := url.Parse(address)
	if query.Query().Get("timestamp") != "1599360473123" ||
		query.Query().Get("sign") != "pEjkZRqG7lUkn9HfMW1l4SPCxDUX99L1ZRjmRkKX53k=" ||
		query.Query().Get("access_token") != "fixture+token" {
		t.Fatal("DingTalk signature, timestamp or token changed")
	}
	if strings.Contains(address, config.Secret) {
		t.Fatal("secret leaked into URL")
	}
	config.Provider = Feishu
	body = `{"msg_type":"text","content":{"text":"hello"},"timestamp":"old","sign":"old"}`
	address, sent, err = Request(config, body, fixedTime)
	if err != nil || address != config.Url {
		t.Fatalf("request failed: %v", err)
	}
	var object map[string]interface{}
	json.Unmarshal([]byte(sent), &object)
	if object["timestamp"] != "1599360473" || object["sign"] != "HIcEaU1WCovPqjRiRKRolPDXnB9tveoDAg/OMhGINHc=" {
		t.Fatal("Feishu signature or seconds timestamp is wrong")
	}
	if strings.Contains(sent, config.Secret) {
		t.Fatal("secret leaked into payload")
	}
}

func TestResolvePreservesSecretsAndRejectsInvalidConfiguration(t *testing.T) {
	current := Config{Provider: DingTalk, SignEnabled: true, Secret: "fixture-secret"}
	input := Update{Url: "https://example.invalid/robot", Template: `{"msgtype":"text","text":{"content":"{{.Result}}"}}`}
	config, err := Resolve(current, input)
	if err != nil || config.Secret != current.Secret || !config.SignEnabled || !config.HasSecret {
		t.Fatalf("old clients should preserve signing settings: %v", err)
	}
	encoded, _ := json.Marshal(config)
	if strings.Contains(string(encoded), "fixture-secret") || strings.Contains(string(encoded), `"secret":`) {
		t.Fatal("API response exposes secret")
	}
	input.SignEnabled = "0"
	input.ClearSecret = true
	config, err = Resolve(current, input)
	if err != nil || config.Secret != "" || config.HasSecret || config.SignEnabled {
		t.Fatalf("explicit clear failed: %v", err)
	}
	for _, invalid := range []Update{
		{Url: input.Url, Template: input.Template, Provider: "unknown"},
		{Url: "javascript:invalid", Template: input.Template},
		{Url: input.Url, Template: "not json"},
		{Url: input.Url, Template: `{"msgtype":"text"}`, SignEnabled: "1", ClearSecret: true},
		{Url: input.Url, Template: `{"msg_type":"text"}`, Provider: Feishu, SignEnabled: "1"},
		{Url: input.Url, Template: `{"text":"generic"}`, Provider: Generic, SignEnabled: "1"},
		{Url: input.Url, Template: input.Template, Secret: "bad\nsecret"},
		{Url: input.Url, Template: input.Template, Secret: strings.Repeat("s", 513)},
	} {
		if _, err := Resolve(current, invalid); err == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
}

func TestRenderEscapesJSONWithoutMutatingTheMessage(t *testing.T) {
	original := "line1\n\"quoted\" <tag> & \\ path"
	message := map[string]interface{}{"name": original, "output": original, "remark": original, "task_id": 42, "status": "成功"}
	body, err := Render(`{"name":"{{.TaskName}}","output":"{{.Result}}","remark":"{{.Remark}}","id":{{.TaskId}}}`, message)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(body), &result)
	if result["name"] != original || result["output"] != original || result["remark"] != original || result["id"] != float64(42) {
		t.Fatal("template escaped values incorrectly")
	}
	if message["name"] != original || message["output"] != original {
		t.Fatal("notification message was mutated")
	}
	for _, source := range []string{`{{`, `{"x":"{{.Missing}}"`, `not-json`} {
		if _, err := Render(source, message); err == nil {
			t.Fatal("invalid template accepted")
		}
	}
}

func TestResponseChecksBusinessCodesAndGenericCompatibility(t *testing.T) {
	for _, item := range []struct{ provider, body string }{
		{Generic, ""}, {DingTalk, `{"errcode":0}`}, {Feishu, `{"code":0}`}, {Feishu, `{"StatusCode":0}`},
	} {
		if err := CheckResponse(item.provider, 200, item.body); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct{ provider, body string }{
		{DingTalk, `{"errcode":310000}`}, {Feishu, `{"code":19021,"msg":"fixture-secret"}`},
		{Feishu, `{"code":19021,"StatusCode":0}`}, {Feishu, `{}`}, {DingTalk, `not-json`},
	} {
		err := CheckResponse(item.provider, 200, item.body)
		if err == nil || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatal("failure was ignored or secret response content was logged")
		}
	}
	if err := CheckResponse(Generic, 500, "private URL"); err == nil || strings.Contains(err.Error(), "private URL") {
		t.Fatal("HTTP failure checking is unsafe")
	}
	body := `{"arbitrary":["legacy",true]}`
	address, sent, err := Request(Config{Url: "https://example.invalid/legacy", Provider: Generic}, body, fixedTime)
	if err != nil || sent != body || address != "https://example.invalid/legacy" {
		t.Fatal("legacy unsigned webhook changed")
	}
}
