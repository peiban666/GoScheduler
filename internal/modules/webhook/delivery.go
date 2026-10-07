package webhook

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Post is shared by task notifications and explicit tests. Redirects are not
// followed: robot credentials must never be forwarded to another endpoint.
func Post(address, body string, timeout int) (int, string) {
	request, err := http.NewRequest(http.MethodPost, address, strings.NewReader(body))
	if err != nil {
		return 0, ""
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "golang/goscheduler")
	client := &http.Client{
		Timeout:       time.Duration(timeout) * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, ""
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil || len(content) > 1024*1024 {
		return 0, ""
	}
	return response.StatusCode, string(content)
}

// TestSend sends exactly one identifiable message without saving configuration.
func TestSend(config Config) error {
	body, err := Render(config.Template, map[string]interface{}{
		"task_id": 0, "name": "Webhook 测试", "status": "测试",
		"output": "这是一条 goscheduler 测试通知，收到表示 Webhook 连接及加签验证成功。",
		"remark": "手动测试发送，不执行任务",
	})
	if err != nil {
		return err
	}
	address, content, err := Request(config, body, time.Now())
	if err != nil {
		return err
	}
	status, result := Post(address, content, 8)
	if status == 0 {
		return fmt.Errorf("发送失败：连接错误、超时或响应读取失败，请检查 URL 和网络")
	}
	return CheckResponse(config.Provider, status, result)
}
