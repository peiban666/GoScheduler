package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
)

const (
	Generic  = "generic"
	DingTalk = "dingtalk"
	Feishu   = "feishu"
)

type Config struct {
	Url         string `json:"url"`
	Template    string `json:"template"`
	Provider    string `json:"provider"`
	SignEnabled bool   `json:"sign_enabled"`
	Secret      string `json:"-"`
	HasSecret   bool   `json:"has_secret"`
}

// Empty provider/sign flags preserve settings when an older client saves.
type Update struct {
	Url, Template, Provider, SignEnabled, Secret string
	ClearSecret                                  bool
}

func Resolve(current Config, input Update) (Config, error) {
	next := Config{Url: strings.TrimSpace(input.Url), Template: strings.TrimSpace(input.Template),
		Provider: strings.TrimSpace(input.Provider)}
	if current.Provider == "" {
		current.Provider = Generic
	}
	if next.Provider == "" {
		next.Provider = current.Provider
	}
	switch next.Provider {
	case Generic, DingTalk, Feishu:
	default:
		return Config{}, fmt.Errorf("请选择有效的 Webhook 平台")
	}
	if next.Provider == current.Provider {
		next.Secret = current.Secret
		next.SignEnabled = current.SignEnabled
	}
	switch input.SignEnabled {
	case "":
	case "1", "true":
		next.SignEnabled = true
	case "0", "false":
		next.SignEnabled = false
	default:
		return Config{}, fmt.Errorf("加签选项无效")
	}
	if input.ClearSecret {
		next.Secret = ""
	} else if secret := strings.TrimSpace(input.Secret); secret != "" {
		next.Secret = secret
	}
	if len(next.Secret) > 512 || strings.ContainsAny(next.Secret, "\r\n\x00") {
		return Config{}, fmt.Errorf("签名密钥格式无效")
	}
	if next.Provider == Generic && next.SignEnabled {
		return Config{}, fmt.Errorf("加签请选择钉钉或飞书平台")
	}
	if next.SignEnabled && next.Secret == "" {
		return Config{}, fmt.Errorf("开启加签后请填写签名密钥")
	}
	address, err := url.Parse(next.Url)
	if err != nil || address.Host == "" || (address.Scheme != "http" && address.Scheme != "https") {
		return Config{}, fmt.Errorf("请输入有效的 HTTP 或 HTTPS Webhook URL")
	}
	if utf8.RuneCountInString(next.Template) == 0 || utf8.RuneCountInString(next.Template) > 4096 {
		return Config{}, fmt.Errorf("通知模板不能为空，且最多 4096 个字符")
	}
	body, err := Render(next.Template, map[string]interface{}{
		"task_id": 1, "name": "任务", "status": "成功", "output": "结果", "remark": "备注",
	})
	if err != nil {
		return Config{}, err
	}
	if err := validateBody(next.Provider, body); err != nil {
		return Config{}, err
	}
	next.HasSecret = next.Secret != ""
	return next, nil
}

// Escape values for JSON without HTML escaping or mutating the notification.
func Render(source string, message map[string]interface{}) (string, error) {
	tmpl, err := template.New("webhook").Parse(source)
	if err != nil {
		return "", fmt.Errorf("Webhook 模板语法无效")
	}
	values := make(map[string]interface{})
	for target, field := range map[string]string{
		"TaskId": "task_id", "TaskName": "name", "Status": "status", "Result": "output", "Remark": "remark",
	} {
		value := ""
		if message[field] != nil {
			value = fmt.Sprint(message[field])
		}
		encoded, _ := json.Marshal(value)
		values[target] = string(encoded[1 : len(encoded)-1])
	}
	var result bytes.Buffer
	if err := tmpl.Execute(&result, values); err != nil {
		return "", fmt.Errorf("Webhook 模板执行失败")
	}
	if !json.Valid(result.Bytes()) {
		return "", fmt.Errorf("Webhook 模板生成的内容不是有效 JSON")
	}
	return result.String(), nil
}

func validateBody(provider, body string) error {
	if !json.Valid([]byte(body)) {
		return fmt.Errorf("Webhook 消息必须是有效 JSON")
	}
	if provider == "" || provider == Generic {
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &object); err != nil || object == nil {
		return fmt.Errorf("机器人消息必须是 JSON 对象")
	}
	key := "msgtype"
	if provider == Feishu {
		key = "msg_type"
	}
	var messageType string
	if err := json.Unmarshal(object[key], &messageType); err != nil || messageType == "" {
		return fmt.Errorf("当前平台的消息模板需要 %s 字段，请使用对应平台模板", key)
	}
	return nil
}

func Request(config Config, body string, now time.Time) (string, string, error) {
	if config.Provider == "" {
		config.Provider = Generic
	}
	if config.Provider != Generic && config.Provider != DingTalk && config.Provider != Feishu {
		return "", "", fmt.Errorf("Webhook 平台无效")
	}
	if err := validateBody(config.Provider, body); err != nil {
		return "", "", err
	}
	if !config.SignEnabled {
		return config.Url, body, nil
	}
	if config.Secret == "" {
		return "", "", fmt.Errorf("签名密钥为空")
	}
	switch config.Provider {
	case DingTalk:
		timestamp := strconv.FormatInt(now.UnixNano()/int64(time.Millisecond), 10)
		digest := hmac.New(sha256.New, []byte(config.Secret))
		digest.Write([]byte(timestamp + "\n" + config.Secret))
		address, err := url.Parse(config.Url)
		if err != nil || address.Host == "" || (address.Scheme != "http" && address.Scheme != "https") {
			return "", "", fmt.Errorf("Webhook URL 无效")
		}
		query := address.Query()
		query.Set("timestamp", timestamp)
		query.Set("sign", base64.StdEncoding.EncodeToString(digest.Sum(nil)))
		address.RawQuery = query.Encode()
		return address.String(), body, nil
	case Feishu:
		timestamp := strconv.FormatInt(now.Unix(), 10)
		// Feishu signs the empty message, using timestamp + "\n" + secret as key.
		digest := hmac.New(sha256.New, []byte(timestamp+"\n"+config.Secret))
		var object map[string]json.RawMessage
		json.Unmarshal([]byte(body), &object)
		object["timestamp"], _ = json.Marshal(timestamp)
		object["sign"], _ = json.Marshal(base64.StdEncoding.EncodeToString(digest.Sum(nil)))
		result, _ := json.Marshal(object)
		return config.Url, string(result), nil
	default:
		return "", "", fmt.Errorf("当前平台未定义加签协议")
	}
}

// HTTP 200 alone does not mean a robot accepted the notification.
func CheckResponse(provider string, status int, body string) error {
	if status < 200 || status >= 300 {
		return fmt.Errorf("Webhook HTTP 状态码 %d", status)
	}
	if provider == Generic || provider == "" {
		return nil
	}
	var result struct {
		ErrCode    *int `json:"errcode"`
		Code       *int `json:"code"`
		StatusCode *int `json:"StatusCode"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return fmt.Errorf("机器人响应不是有效 JSON")
	}
	code := result.ErrCode
	if provider == Feishu {
		code = result.Code
		if code == nil {
			code = result.StatusCode
		}
	}
	if code == nil {
		return fmt.Errorf("机器人响应缺少结果码")
	}
	if *code != 0 {
		return fmt.Errorf("机器人返回错误码 %d", *code)
	}
	return nil
}
