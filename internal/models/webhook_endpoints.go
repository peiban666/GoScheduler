package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gaggad/goscheduler/internal/modules/webhook"
)

const webhookEndpointCode = "webhook_endpoint"

// Each endpoint has an identity row, plus separate URL/template rows. This keeps
// the existing setting schema and its 4096-character value limit compatible.
type WebhookEndpoint struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	webhook.Config
}

type webhookEndpointMetadata struct {
	Name string `json:"name"`
	webhookSigning
}

func webhookSQL(statement string) string {
	for _, filter := range Db.Dialect().Filters() {
		statement = filter.Do(statement, Db.Dialect(), nil)
	}
	return statement
}

func webhookTable() string { return Db.Quote(Db.TableName(new(Setting))) }

func normalizeWebhookName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return "", fmt.Errorf("Webhook 名称不能为空，且最多 64 个字符")
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("Webhook 名称请使用单行文本")
		}
	}
	return name, nil
}

func (setting *Setting) WebhookEndpoints() ([]WebhookEndpoint, error) {
	endpoints := make([]WebhookEndpoint, 0)
	legacy, name, err := setting.legacyWebhook()
	if err != nil {
		return nil, err
	}
	if legacy.Url != "" {
		endpoints = append(endpoints, WebhookEndpoint{Id: 0, Name: name, Config: legacy})
	}
	// Do not run secret-bearing reads through xorm's SQL argument/result logger.
	rows, err := Db.DB().DB.Query(webhookSQL("SELECT id, "+Db.Quote("key")+", value FROM "+webhookTable()+" WHERE code = ? ORDER BY id"),
		webhookEndpointCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]string)
	identities := make([]Setting, 0)
	for rows.Next() {
		var row Setting
		if err := rows.Scan(&row.Id, &row.Key, &row.Value); err != nil {
			return nil, err
		}
		if row.Key == "endpoint" {
			identities = append(identities, row)
		} else {
			values[row.Key] = row.Value
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, row := range identities {
		var metadata webhookEndpointMetadata
		if err := json.Unmarshal([]byte(row.Value), &metadata); err != nil {
			return nil, fmt.Errorf("Webhook 配置读取失败")
		}
		suffix := ":" + strconv.Itoa(row.Id)
		endpoints = append(endpoints, WebhookEndpoint{Id: row.Id, Name: metadata.Name, Config: webhook.Config{
			Url: values["url"+suffix], Template: values["template"+suffix],
			Provider: metadata.Provider, SignEnabled: metadata.SignEnabled, Secret: metadata.Secret, HasSecret: metadata.Secret != "",
		}})
	}
	return endpoints, nil
}

func (setting *Setting) WebhookEndpoint(id int) (WebhookEndpoint, error) {
	if id < 0 {
		return WebhookEndpoint{}, fmt.Errorf("请选择有效的 Webhook")
	}
	endpoints, err := setting.WebhookEndpoints()
	if err != nil {
		return WebhookEndpoint{}, fmt.Errorf("读取 Webhook 配置失败")
	}
	for _, endpoint := range endpoints {
		if endpoint.Id == id {
			return endpoint, nil
		}
	}
	return WebhookEndpoint{}, fmt.Errorf("Webhook 不存在，请重新选择")
}

// id == -1 creates an endpoint; id == 0 edits the legacy default.
func (setting *Setting) SaveWebhookEndpoint(id int, name string, input webhook.Update) (WebhookEndpoint, error) {
	webhookSettingsMu.Lock()
	defer webhookSettingsMu.Unlock()
	if id < -1 {
		return WebhookEndpoint{}, fmt.Errorf("Webhook 编号无效")
	}
	name, err := normalizeWebhookName(name)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	current := WebhookEndpoint{Config: webhook.Config{Provider: webhook.Generic}}
	if id >= 0 {
		current, err = setting.WebhookEndpoint(id)
		if err != nil {
			return WebhookEndpoint{}, err
		}
	}
	config, err := webhook.Resolve(current.Config, input)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	if id == 0 {
		if err := setting.updateWebhook(config, &name); err != nil {
			return WebhookEndpoint{}, fmt.Errorf("保存 Webhook 配置失败")
		}
		return WebhookEndpoint{Id: 0, Name: name, Config: config}, nil
	}
	data, err := json.Marshal(webhookEndpointMetadata{name, webhookSigning{config.Provider, config.SignEnabled, config.Secret}})
	if err != nil {
		return WebhookEndpoint{}, fmt.Errorf("Webhook 配置无效")
	}
	tx, err := Db.DB().DB.Begin()
	if err != nil {
		return WebhookEndpoint{}, fmt.Errorf("保存 Webhook 配置失败")
	}
	defer tx.Rollback()
	table := webhookTable()
	if id == -1 {
		statement := "INSERT INTO " + table + " (code, " + Db.Quote("key") + ", value) VALUES (?, ?, ?)"
		if Db.DriverName() == "postgres" {
			err = tx.QueryRow(webhookSQL(statement+" RETURNING id"), webhookEndpointCode, "endpoint", string(data)).Scan(&id)
		} else {
			var result sql.Result
			result, err = tx.Exec(webhookSQL(statement), webhookEndpointCode, "endpoint", string(data))
			if err == nil {
				var inserted int64
				inserted, err = result.LastInsertId()
				id = int(inserted)
			}
		}
	} else {
		var result sql.Result
		result, err = tx.Exec(webhookSQL("UPDATE "+table+" SET value = ? WHERE id = ? AND code = ? AND "+Db.Quote("key")+" = ?"),
			string(data), id, webhookEndpointCode, "endpoint")
		_ = result
	}
	if err != nil {
		return WebhookEndpoint{}, fmt.Errorf("保存 Webhook 配置失败")
	}
	for _, entry := range []struct{ key, value string }{{"url", config.Url}, {"template", config.Template}} {
		key := entry.key + ":" + strconv.Itoa(id)
		var rowID int
		err = tx.QueryRow(webhookSQL("SELECT id FROM "+table+" WHERE code = ? AND "+Db.Quote("key")+" = ? LIMIT 1 FOR UPDATE"),
			webhookEndpointCode, key).Scan(&rowID)
		if err == sql.ErrNoRows {
			_, err = tx.Exec(webhookSQL("INSERT INTO "+table+" (code, "+Db.Quote("key")+", value) VALUES (?, ?, ?)"),
				webhookEndpointCode, key, entry.value)
		} else if err == nil {
			_, err = tx.Exec(webhookSQL("UPDATE "+table+" SET value = ? WHERE code = ? AND "+Db.Quote("key")+" = ?"),
				entry.value, webhookEndpointCode, key)
		}
		if err != nil {
			return WebhookEndpoint{}, fmt.Errorf("保存 Webhook 配置失败")
		}
	}
	if err := tx.Commit(); err != nil {
		return WebhookEndpoint{}, fmt.Errorf("保存 Webhook 配置失败")
	}
	return WebhookEndpoint{Id: id, Name: name, Config: config}, nil
}

func ParseWebhookIDs(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		// Old tasks used the global webhook without a receiver ID.
		return []int{0}, nil
	}
	ids, seen := make([]int, 0), make(map[int]bool)
	if len(value) > 256 {
		return nil, fmt.Errorf("Webhook 选择数量过多")
	}
	for _, token := range strings.Split(value, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(token))
		if err != nil || id < 0 {
			return nil, fmt.Errorf("请选择有效的 Webhook")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

func (setting *Setting) WebhooksForTask(value string) ([]WebhookEndpoint, error) {
	ids, err := ParseWebhookIDs(value)
	if err != nil {
		return nil, err
	}
	list, err := setting.WebhookEndpoints()
	if err != nil {
		return nil, fmt.Errorf("读取 Webhook 配置失败")
	}
	index := make(map[int]WebhookEndpoint, len(list))
	for _, endpoint := range list {
		index[endpoint.Id] = endpoint
	}
	result := make([]WebhookEndpoint, 0, len(ids))
	for _, id := range ids {
		endpoint, exists := index[id]
		if !exists {
			return nil, fmt.Errorf("所选 Webhook 已删除或尚未配置，请重新选择")
		}
		result = append(result, endpoint)
	}
	return result, nil
}

func (setting *Setting) RemoveWebhookEndpoint(id int) error {
	webhookSettingsMu.Lock()
	defer webhookSettingsMu.Unlock()
	if _, err := setting.WebhookEndpoint(id); err != nil {
		return err
	}
	tasks := make([]Task, 0)
	if err := Db.Where("notify_type = ?", 3).Cols("id", "notify_receiver_id").Find(&tasks); err != nil {
		return fmt.Errorf("读取任务通知配置失败")
	}
	for _, task := range tasks {
		ids, err := ParseWebhookIDs(task.NotifyReceiverId)
		if err != nil {
			return fmt.Errorf("任务通知配置无效，请先修正任务")
		}
		for _, selected := range ids {
			if selected == id {
				return fmt.Errorf("任务 %d 正在使用此 Webhook，请先修改任务通知设置", task.Id)
			}
		}
	}
	tx, err := Db.DB().DB.Begin()
	if err != nil {
		return fmt.Errorf("删除 Webhook 失败")
	}
	defer tx.Rollback()
	if id == 0 {
		_, err = tx.Exec(webhookSQL("DELETE FROM "+webhookTable()+" WHERE code = ?"), WebhookCode)
	} else {
		_, err = tx.Exec(webhookSQL("DELETE FROM "+webhookTable()+" WHERE code = ? AND (id = ? OR "+Db.Quote("key")+" IN (?, ?))"),
			webhookEndpointCode, id, "url:"+strconv.Itoa(id), "template:"+strconv.Itoa(id))
	}
	if err != nil || tx.Commit() != nil {
		return fmt.Errorf("删除 Webhook 失败")
	}
	return nil
}
