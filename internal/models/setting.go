package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gaggad/goscheduler/internal/modules/webhook"
)

type Setting struct {
	Id    int    `xorm:"int pk autoincr"`
	Code  string `xorm:"varchar(32) notnull"`
	Key   string `xorm:"varchar(64) notnull"`
	Value string `xorm:"varchar(4096) notnull default '' "`
}

const slackTemplate = `
任务ID:  {{.TaskId}}
任务名称: {{.TaskName}}
状态:    {{.Status}}
执行结果: {{.Result}}
备注: {{.Remark}}
`
const emailTemplate = `
任务ID:  {{.TaskId}}
任务名称: {{.TaskName}}
状态:    {{.Status}}
执行结果: {{.Result}}
备注: {{.Remark}}
`
const webhookTemplate = `
{
  "task_id": "{{.TaskId}}",
  "task_name": "{{.TaskName}}",
  "status": "{{.Status}}",
  "result": "{{.Result}}",
  "remark": "{{.Remark}}"
}
`

const (
	SlackCode        = "slack"
	SlackUrlKey      = "url"
	SlackTemplateKey = "template"
	SlackChannelKey  = "channel"
)

const (
	MailCode        = "mail"
	MailTemplateKey = "template"
	MailServerKey   = "server"
	MailUserKey     = "user"
)

const (
	WebhookCode        = "webhook"
	WebhookTemplateKey = "template"
	WebhookUrlKey      = "url"
	WebhookSigningKey  = "signing"
	WebhookNameKey     = "name"
)

// 初始化基本字段 邮件、slack等
func (setting *Setting) InitBasicField() {
	setting.Code = SlackCode
	setting.Key = SlackUrlKey
	setting.Value = ""
	Db.Insert(setting)
	setting.Id = 0

	setting.Code = SlackCode
	setting.Key = SlackTemplateKey
	setting.Value = slackTemplate
	Db.Insert(setting)
	setting.Id = 0

	setting.Code = MailCode
	setting.Key = MailServerKey
	setting.Value = ""
	Db.Insert(setting)
	setting.Id = 0

	setting.Code = MailCode
	setting.Key = MailTemplateKey
	setting.Value = emailTemplate
	Db.Insert(setting)
	setting.Id = 0

	setting.Code = WebhookCode
	setting.Key = WebhookTemplateKey
	setting.Value = webhookTemplate
	Db.Insert(setting)
	setting.Id = 0

	setting.Code = WebhookCode
	setting.Key = WebhookUrlKey
	setting.Value = ""
	Db.Insert(setting)
}

// region slack配置

type Slack struct {
	Url      string    `json:"url"`
	Channels []Channel `json:"channels"`
	Template string    `json:"template"`
}

type Channel struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func (setting *Setting) Slack() (Slack, error) {
	list := make([]Setting, 0)
	err := Db.Where("code = ?", SlackCode).Find(&list)
	slack := Slack{}
	if err != nil {
		return slack, err
	}

	setting.formatSlack(list, &slack)

	return slack, err
}

func (setting *Setting) formatSlack(list []Setting, slack *Slack) {
	for _, v := range list {
		switch v.Key {
		case SlackUrlKey:
			slack.Url = v.Value
		case SlackTemplateKey:
			slack.Template = v.Value
		default:
			slack.Channels = append(slack.Channels, Channel{
				v.Id, v.Value,
			})
		}
	}
}

func (setting *Setting) UpdateSlack(url, template string) error {
	setting.Value = url

	Db.Cols("value").Update(setting, Setting{Code: SlackCode, Key: SlackUrlKey})

	setting.Value = template
	Db.Cols("value").Update(setting, Setting{Code: SlackCode, Key: SlackTemplateKey})

	return nil
}

// 创建slack渠道
func (setting *Setting) CreateChannel(channel string) (int64, error) {
	setting.Code = SlackCode
	setting.Key = SlackChannelKey
	setting.Value = channel

	return Db.Insert(setting)
}

func (setting *Setting) IsChannelExist(channel string) bool {
	setting.Code = SlackCode
	setting.Key = SlackChannelKey
	setting.Value = channel

	count, _ := Db.Count(setting)

	return count > 0
}

// 删除slack渠道
func (setting *Setting) RemoveChannel(id int) (int64, error) {
	setting.Code = SlackCode
	setting.Key = SlackChannelKey
	setting.Id = id
	return Db.Delete(setting)
}

// endregion

type Mail struct {
	Host      string     `json:"host"`
	Port      int        `json:"port"`
	User      string     `json:"user"`
	Password  string     `json:"password"`
	MailUsers []MailUser `json:"mail_users"`
	Template  string     `json:"template"`
}

type MailUser struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// region 邮件配置
func (setting *Setting) Mail() (Mail, error) {
	list := make([]Setting, 0)
	err := Db.Where("code = ?", MailCode).Find(&list)
	mail := Mail{MailUsers: make([]MailUser, 0)}
	if err != nil {
		return mail, err
	}

	setting.formatMail(list, &mail)

	return mail, err
}

func (setting *Setting) formatMail(list []Setting, mail *Mail) {
	mailUser := MailUser{}
	for _, v := range list {
		switch v.Key {
		case MailServerKey:
			json.Unmarshal([]byte(v.Value), mail)
		case MailUserKey:
			json.Unmarshal([]byte(v.Value), &mailUser)
			mailUser.Id = v.Id
			mail.MailUsers = append(mail.MailUsers, mailUser)
		case MailTemplateKey:
			mail.Template = v.Value
		}

	}
}

func (setting *Setting) UpdateMail(config, template string) error {
	setting.Value = config
	Db.Cols("value").Update(setting, Setting{Code: MailCode, Key: MailServerKey})

	setting.Value = template
	Db.Cols("value").Update(setting, Setting{Code: MailCode, Key: MailTemplateKey})

	return nil
}

func (setting *Setting) CreateMailUser(username, email string) (int64, error) {
	setting.Code = MailCode
	setting.Key = MailUserKey
	mailUser := MailUser{0, username, email}
	jsonByte, err := json.Marshal(mailUser)
	if err != nil {
		return 0, err
	}
	setting.Value = string(jsonByte)

	return Db.Insert(setting)
}

func (setting *Setting) RemoveMailUser(id int) (int64, error) {
	setting.Code = MailCode
	setting.Key = MailUserKey
	setting.Id = id
	return Db.Delete(setting)
}

type WebHook = webhook.Config

type webhookSigning struct {
	Provider    string `json:"provider"`
	SignEnabled bool   `json:"sign_enabled"`
	Secret      string `json:"secret"`
}

var webhookSettingsMu sync.Mutex

func (setting *Setting) SaveWebhook(input webhook.Update) error {
	webhookSettingsMu.Lock()
	defer webhookSettingsMu.Unlock()
	current, err := setting.Webhook()
	if err != nil {
		return fmt.Errorf("读取 Webhook 配置失败")
	}
	config, err := webhook.Resolve(current, input)
	if err != nil {
		return err
	}
	if err := setting.UpdateWebHook(config); err != nil {
		return fmt.Errorf("保存 Webhook 配置失败")
	}
	return nil
}

func (setting *Setting) Webhook() (WebHook, error) {
	config, _, err := setting.legacyWebhook()
	return config, err
}

func (setting *Setting) legacyWebhook() (WebHook, string, error) {
	list := make([]Setting, 0)
	err := Db.Where("code = ?", WebhookCode).Find(&list)
	webHook := WebHook{Provider: webhook.Generic}
	name := "默认 Webhook"
	if err != nil {
		return webHook, name, err
	}
	for _, entry := range list {
		if entry.Key == WebhookNameKey && strings.TrimSpace(entry.Value) != "" {
			name = entry.Value
		}
	}

	err = setting.formatWebhook(list, &webHook)
	return webHook, name, err
}

func (setting *Setting) formatWebhook(list []Setting, webHook *WebHook) error {
	for _, v := range list {
		switch v.Key {
		case WebhookUrlKey:
			webHook.Url = v.Value
		case WebhookTemplateKey:
			webHook.Template = v.Value
		case WebhookSigningKey:
			var signing webhookSigning
			if err := json.Unmarshal([]byte(v.Value), &signing); err != nil {
				return err
			}
			webHook.Provider = signing.Provider
			webHook.SignEnabled = signing.SignEnabled
			webHook.Secret = signing.Secret
		}
	}
	if webHook.Provider == "" {
		webHook.Provider = webhook.Generic
	}
	webHook.HasSecret = webHook.Secret != ""
	return nil
}

func (setting *Setting) UpdateWebHook(config WebHook) error {
	return setting.updateWebhook(config, nil)
}

func (setting *Setting) updateWebhook(config WebHook, name *string) error {
	signing, err := json.Marshal(webhookSigning{config.Provider, config.SignEnabled, config.Secret})
	if err != nil {
		return err
	}
	// Parameterized database/sql avoids xorm's development SQL argument logging:
	// the robot URL can contain a token and the signing metadata contains a secret.
	tx, err := Db.DB().DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	table := Db.Quote(Db.TableName(new(Setting)))
	sqlForDialect := func(statement string) string {
		for _, filter := range Db.Dialect().Filters() {
			statement = filter.Do(statement, Db.Dialect(), nil)
		}
		return statement
	}
	entries := []struct{ key, value string }{
		{WebhookUrlKey, config.Url}, {WebhookTemplateKey, config.Template}, {WebhookSigningKey, string(signing)},
	}
	if name != nil {
		entries = append(entries, struct{ key, value string }{WebhookNameKey, *name})
	}
	for _, entry := range entries {
		var id int
		err := tx.QueryRow(sqlForDialect("SELECT id FROM "+table+" WHERE code = ? AND "+Db.Quote("key")+" = ? LIMIT 1 FOR UPDATE"),
			WebhookCode, entry.key).Scan(&id)
		if err == sql.ErrNoRows {
			_, err = tx.Exec(sqlForDialect("INSERT INTO "+table+" (code, "+Db.Quote("key")+", value) VALUES (?, ?, ?)"),
				WebhookCode, entry.key, entry.value)
		} else if err == nil {
			_, err = tx.Exec(sqlForDialect("UPDATE "+table+" SET value = ? WHERE code = ? AND "+Db.Quote("key")+" = ?"),
				entry.value, WebhookCode, entry.key)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// endregion
