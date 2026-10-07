package manage

import (
	"encoding/json"
	"strconv"

	"github.com/gaggad/goscheduler/internal/models"
	"github.com/gaggad/goscheduler/internal/modules/logger"
	"github.com/gaggad/goscheduler/internal/modules/utils"
	"github.com/gaggad/goscheduler/internal/modules/webhook"
	"gopkg.in/macaron.v1"
)

func Slack(ctx *macaron.Context) string {
	settingModel := new(models.Setting)
	slack, err := settingModel.Slack()
	jsonResp := utils.JsonResponse{}
	if err != nil {
		logger.Error(err)
		return jsonResp.Success(utils.SuccessContent, nil)

	}

	return jsonResp.Success(utils.SuccessContent, slack)
}

func UpdateSlack(ctx *macaron.Context) string {
	url := ctx.QueryTrim("url")
	template := ctx.QueryTrim("template")
	settingModel := new(models.Setting)
	err := settingModel.UpdateSlack(url, template)

	return utils.JsonResponseByErr(err)
}

func CreateSlackChannel(ctx *macaron.Context) string {
	channel := ctx.QueryTrim("channel")
	settingModel := new(models.Setting)
	if settingModel.IsChannelExist(channel) {
		jsonResp := utils.JsonResponse{}

		return jsonResp.CommonFailure("Channel已存在")
	}
	_, err := settingModel.CreateChannel(channel)

	return utils.JsonResponseByErr(err)
}

func RemoveSlackChannel(ctx *macaron.Context) string {
	id := ctx.ParamsInt(":id")
	settingModel := new(models.Setting)
	_, err := settingModel.RemoveChannel(id)

	return utils.JsonResponseByErr(err)
}

// endregion

// region 邮件
func Mail(ctx *macaron.Context) string {
	settingModel := new(models.Setting)
	mail, err := settingModel.Mail()
	jsonResp := utils.JsonResponse{}
	if err != nil {
		logger.Error(err)
		return jsonResp.Success(utils.SuccessContent, nil)
	}

	return jsonResp.Success("", mail)
}

type MailServerForm struct {
	Host     string `binding:"Required;MaxSize(100)"`
	Port     int    `binding:"Required;Range(1-65535)"`
	User     string `binding:"Required;MaxSize(64);Email"`
	Password string `binding:"Required;MaxSize(64)"`
}

func UpdateMail(ctx *macaron.Context, form MailServerForm) string {
	jsonByte, _ := json.Marshal(form)
	settingModel := new(models.Setting)

	template := ctx.QueryTrim("template")
	err := settingModel.UpdateMail(string(jsonByte), template)

	return utils.JsonResponseByErr(err)
}

func CreateMailUser(ctx *macaron.Context) string {
	username := ctx.QueryTrim("username")
	email := ctx.QueryTrim("email")
	settingModel := new(models.Setting)
	if username == "" || email == "" {
		jsonResp := utils.JsonResponse{}

		return jsonResp.CommonFailure("用户名、邮箱均不能为空")
	}
	_, err := settingModel.CreateMailUser(username, email)

	return utils.JsonResponseByErr(err)
}

func RemoveMailUser(ctx *macaron.Context) string {
	id := ctx.ParamsInt(":id")
	settingModel := new(models.Setting)
	_, err := settingModel.RemoveMailUser(id)

	return utils.JsonResponseByErr(err)
}

func WebHook(ctx *macaron.Context) string {
	settingModel := new(models.Setting)
	webHook, err := settingModel.Webhook()
	jsonResp := utils.JsonResponse{}
	if err != nil {
		return jsonResp.CommonFailure("读取 Webhook 配置失败")
	}

	return jsonResp.Success("", webHook)
}

func webhookUpdate(ctx *macaron.Context) webhook.Update {
	return webhook.Update{
		Url: ctx.Query("url"), Template: ctx.Query("template"), Provider: ctx.Query("provider"),
		SignEnabled: ctx.Query("sign_enabled"), Secret: ctx.Query("secret"),
		ClearSecret: ctx.Query("clear_secret") == "1" || ctx.Query("clear_secret") == "true",
	}
}

func UpdateWebHook(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	settingModel := new(models.Setting)
	err := settingModel.SaveWebhook(webhookUpdate(ctx))
	if err != nil {
		return json.CommonFailure(err.Error())
	}
	return json.Success("Webhook 配置已保存", nil)
}

func WebhookList(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	list, err := new(models.Setting).WebhookEndpoints()
	if err != nil {
		return json.CommonFailure("读取 Webhook 列表失败")
	}
	return json.Success("", list)
}

func WebhookOptions(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	list, err := new(models.Setting).WebhookEndpoints()
	if err != nil {
		return json.CommonFailure("读取 Webhook 列表失败")
	}
	type option struct {
		Id       int    `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
	}
	options := make([]option, 0, len(list))
	for _, endpoint := range list {
		options = append(options, option{endpoint.Id, endpoint.Name, endpoint.Provider})
	}
	return json.Success("", options)
}

func StoreWebhook(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	id, err := strconv.Atoi(ctx.Query("id"))
	if err != nil {
		return json.CommonFailure("Webhook 编号无效")
	}
	endpoint, err := new(models.Setting).SaveWebhookEndpoint(id, ctx.Query("name"), webhookUpdate(ctx))
	if err != nil {
		return json.CommonFailure(err.Error())
	}
	return json.Success("Webhook 已保存", endpoint)
}

func RemoveWebhook(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	if err := new(models.Setting).RemoveWebhookEndpoint(ctx.ParamsInt(":id")); err != nil {
		return json.CommonFailure(err.Error())
	}
	return json.Success("Webhook 已删除", nil)
}

func TestWebhook(ctx *macaron.Context) string {
	json := utils.JsonResponse{}
	id, err := strconv.Atoi(ctx.Query("id"))
	if err != nil || id < -1 {
		return json.CommonFailure("Webhook 编号无效")
	}
	config := webhook.Config{Provider: webhook.Generic}
	if id >= 0 {
		endpoint, err := new(models.Setting).WebhookEndpoint(id)
		if err != nil {
			return json.CommonFailure(err.Error())
		}
		config = endpoint.Config
	}
	// Tests support unsaved edits, but never mutate stored settings.
	if ctx.Query("url") != "" {
		config, err = webhook.Resolve(config, webhookUpdate(ctx))
		if err != nil {
			return json.CommonFailure(err.Error())
		}
	}
	if config.Url == "" {
		return json.CommonFailure("请先填写 Webhook URL")
	}
	if err := webhook.TestSend(config); err != nil {
		return json.CommonFailure(err.Error())
	}
	return json.Success("测试通知已发送，机器人已确认接收", nil)
}

// endregion
