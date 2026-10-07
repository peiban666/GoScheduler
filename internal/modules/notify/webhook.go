package notify

import (
	"fmt"
	"time"

	"github.com/gaggad/goscheduler/internal/models"
	"github.com/gaggad/goscheduler/internal/modules/httpclient"
	"github.com/gaggad/goscheduler/internal/modules/logger"
	"github.com/gaggad/goscheduler/internal/modules/webhook"
)

type WebHook struct{}

func (webHook *WebHook) Send(msg Message) {
	model := new(models.Setting)
	receiver, _ := msg["task_receiver_id"].(string)
	endpoints, err := model.WebhooksForTask(receiver)
	if err != nil {
		logger.Error("#webHook#读取任务 Webhook 配置失败")
		return
	}
	for _, endpoint := range endpoints {
		body, err := webhook.Render(endpoint.Template, msg)
		if err != nil {
			logger.Errorf("#webHook#通知模板无效#ID=%d", endpoint.Id)
			continue
		}
		if err := sendWebhook(endpoint.Config, body, postWebhook, time.Now, time.Sleep); err != nil {
			// Never print the URL, configuration, response body or secret.
			logger.Errorf("#webHook#发送失败#ID=%d#%s", endpoint.Id, err)
		}
	}
}

func postWebhook(address, body string, timeout int) httpclient.ResponseWrapper {
	status, result := webhook.Post(address, body, timeout)
	return httpclient.ResponseWrapper{StatusCode: status, Body: result}
}

func sendWebhook(config models.WebHook, body string,
	post func(string, string, int) httpclient.ResponseWrapper,
	now func() time.Time, sleep func(time.Duration)) error {
	var lastError error
	for attempt := 0; attempt < 3; attempt++ {
		// Recompute timestamp/sign for each retry, instead of reusing an old sign.
		address, content, err := webhook.Request(config, body, now())
		if err != nil {
			return err
		}
		resp := post(address, content, 30)
		lastError = webhook.CheckResponse(config.Provider, resp.StatusCode, resp.Body)
		if lastError == nil {
			return nil
		}
		if attempt < 2 {
			sleep(2 * time.Second)
		}
	}
	return fmt.Errorf("重试 3 次后仍未成功：%s", lastError)
}
