package models

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gaggad/goscheduler/internal/modules/webhook"
)

func TestWebhookSettingsDefaultsAndMaskedAPI(t *testing.T) {
	for _, signed := range []bool{false, true} {
		mock := mockTaskGroupDB(t)
		rows := sqlmock.NewRows([]string{"code", "key", "value"}).
			AddRow(WebhookCode, WebhookUrlKey, "https://example.invalid/robot").
			AddRow(WebhookCode, WebhookTemplateKey, `{"text":"legacy"}`)
		if signed {
			rows.AddRow(WebhookCode, WebhookSigningKey, `{"provider":"dingtalk","sign_enabled":true,"secret":"fixture-secret"}`)
		}
		mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\?").
			WithArgs(WebhookCode).WillReturnRows(rows)
		config, err := new(Setting).Webhook()
		if err != nil || config.Url == "" || config.SignEnabled != signed || config.HasSecret != signed {
			t.Fatalf("configuration not loaded: %v", err)
		}
		if !signed && config.Provider != webhook.Generic {
			t.Fatal("legacy provider should be generic")
		}
		data, _ := json.Marshal(config)
		if strings.Contains(string(data), "fixture-secret") || strings.Contains(string(data), `"secret":`) {
			t.Fatal("API serializes signing secret")
		}
	}
}

func TestWebhookSettingsUpsertIsAtomicAndPreciselyScoped(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "rollback"}[fail], func(t *testing.T) {
			mock := mockTaskGroupDB(t)
			mock.ExpectBegin()
			config := WebHook{Url: "https://example.invalid/robot", Template: `{"msgtype":"text"}`,
				Provider: webhook.DingTalk, SignEnabled: true, Secret: "fixture-secret"}
			for _, entry := range []struct{ key, value string }{
				{WebhookUrlKey, config.Url}, {WebhookTemplateKey, config.Template},
			} {
				mock.ExpectQuery("SELECT id FROM `setting` WHERE code = \\? AND `key` = \\? LIMIT 1 FOR UPDATE").
					WithArgs(WebhookCode, entry.key).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				mock.ExpectExec("UPDATE `setting` SET value = \\? WHERE code = \\? AND `key` = \\?").
					WithArgs(entry.value, WebhookCode, entry.key).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("SELECT id FROM `setting` WHERE code = \\? AND `key` = \\? LIMIT 1 FOR UPDATE").
				WithArgs(WebhookCode, WebhookSigningKey).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			insert := mock.ExpectExec("INSERT INTO `setting` \\(code, `key`, value\\) VALUES \\(\\?, \\?, \\?\\)").
				WithArgs(WebhookCode, WebhookSigningKey, `{"provider":"dingtalk","sign_enabled":true,"secret":"fixture-secret"}`)
			if fail {
				insert.WillReturnError(errors.New("fixture database failure"))
				mock.ExpectRollback()
			} else {
				insert.WillReturnResult(sqlmock.NewResult(3, 1))
				mock.ExpectCommit()
			}
			err := new(Setting).UpdateWebHook(config)
			if (err != nil) != fail {
				t.Fatalf("unexpected save result: %v", err)
			}
		})
	}
}
