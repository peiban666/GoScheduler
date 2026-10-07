package models

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gaggad/goscheduler/internal/modules/webhook"
)

func expectEndpointList(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\?").WithArgs(WebhookCode).
		WillReturnRows(sqlmock.NewRows([]string{"code", "key", "value"}).
			AddRow(WebhookCode, WebhookUrlKey, "https://example.invalid/legacy").
			AddRow(WebhookCode, WebhookTemplateKey, `{"text":"legacy"}`))
	mock.ExpectQuery("SELECT id, `key`, value FROM `setting` WHERE code = \\? ORDER BY id").
		WithArgs(webhookEndpointCode).WillReturnRows(sqlmock.NewRows([]string{"id", "key", "value"}).
		AddRow(10, "endpoint", `{"name":"运维","provider":"dingtalk","sign_enabled":true,"secret":"fixture-a"}`).
		AddRow(11, "url:10", "https://example.invalid/a").
		AddRow(12, "template:10", `{"msgtype":"text"}`).
		AddRow(13, "endpoint", `{"name":"日报","provider":"feishu","sign_enabled":true,"secret":"fixture-b"}`).
		AddRow(14, "url:13", "https://example.invalid/b").
		AddRow(15, "template:13", `{"msg_type":"text"}`))
}

func TestTaskWebhookSelectionAndNoSecretSerialization(t *testing.T) {
	mock := mockTaskGroupDB(t)
	expectEndpointList(mock)
	list, err := new(Setting).WebhooksForTask("13,10,13")
	if err != nil || len(list) != 2 || list[0].Id != 13 || list[1].Id != 10 || list[0].Secret != "fixture-b" {
		t.Fatalf("wrong endpoints: %v", err)
	}
	data, _ := json.Marshal(list)
	if strings.Contains(string(data), "fixture-") || strings.Contains(string(data), `"secret":`) {
		t.Fatal("secret serialized")
	}
	expectEndpointList(mock)
	legacy, err := new(Setting).WebhooksForTask("")
	if err != nil || len(legacy) != 1 || legacy[0].Id != 0 {
		t.Fatal("legacy notification changed")
	}
	expectEndpointList(mock)
	if _, err := new(Setting).WebhooksForTask("999"); err == nil {
		t.Fatal("missing webhook silently fell back to global config")
	}
}

func TestWebhookIDsAndNames(t *testing.T) {
	for _, value := range []string{"-1", "1,", "x", strings.Repeat("1,", 150)} {
		if _, err := ParseWebhookIDs(value); err == nil {
			t.Fatalf("accepted bad IDs")
		}
	}
	for _, value := range []string{"", "  ", "abc\n123", strings.Repeat("名", 65)} {
		if _, err := normalizeWebhookName(value); err == nil {
			t.Fatal("accepted bad name")
		}
	}
}

func TestWebhookRemovalRejectsTaskReferences(t *testing.T) {
	mock := mockTaskGroupDB(t)
	expectEndpointList(mock)
	mock.ExpectQuery("SELECT .* FROM `task` WHERE .*notify_type = \\?").WithArgs(3, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "notify_receiver_id"}).AddRow(88, "13,10"))
	err := new(Setting).RemoveWebhookEndpoint(10)
	if err == nil || !strings.Contains(err.Error(), "88") {
		t.Fatal("endpoint was removed while referenced by a task")
	}
}

func TestLegacyWebhookRenameIsAtomicAndPreservesSigning(t *testing.T) {
	for _, failName := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved", true: "rollback"}[failName], func(t *testing.T) {
			mock := mockTaskGroupDB(t)
			address := "https://example.invalid/robot"
			template := `{"msgtype":"text","text":{"content":"{{.TaskName}}"}}`
			signing := `{"provider":"dingtalk","sign_enabled":true,"secret":"fixture-secret"}`
			mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\?").WithArgs(WebhookCode).
				WillReturnRows(sqlmock.NewRows([]string{"code", "key", "value"}).
					AddRow(WebhookCode, WebhookUrlKey, address).
					AddRow(WebhookCode, WebhookTemplateKey, template).
					AddRow(WebhookCode, WebhookSigningKey, signing))
			mock.ExpectQuery("SELECT id, `key`, value FROM `setting` WHERE code = \\? ORDER BY id").
				WithArgs(webhookEndpointCode).WillReturnRows(sqlmock.NewRows([]string{"id", "key", "value"}))
			mock.ExpectBegin()
			for _, entry := range []struct{ key, value string }{
				{WebhookUrlKey, address}, {WebhookTemplateKey, template}, {WebhookSigningKey, signing},
			} {
				mock.ExpectQuery("SELECT id FROM `setting` WHERE code = \\? AND `key` = \\? LIMIT 1 FOR UPDATE").
					WithArgs(WebhookCode, entry.key).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				mock.ExpectExec("UPDATE `setting` SET value = \\? WHERE code = \\? AND `key` = \\?").
					WithArgs(entry.value, WebhookCode, entry.key).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("SELECT id FROM `setting` WHERE code = \\? AND `key` = \\? LIMIT 1 FOR UPDATE").
				WithArgs(WebhookCode, WebhookNameKey).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			insert := mock.ExpectExec("INSERT INTO `setting` \\(code, `key`, value\\) VALUES \\(\\?, \\?, \\?\\)").
				WithArgs(WebhookCode, WebhookNameKey, "运维钉钉群")
			if failName {
				insert.WillReturnError(errors.New("fixture failure"))
				mock.ExpectRollback()
			} else {
				insert.WillReturnResult(sqlmock.NewResult(4, 1))
				mock.ExpectCommit()
			}
			result, err := new(Setting).SaveWebhookEndpoint(0, "运维钉钉群", webhook.Update{
				Url: address, Template: template, Provider: "dingtalk", SignEnabled: "1",
			})
			if (err != nil) != failName {
				t.Fatalf("unexpected rename result: %v", err)
			}
			if !failName && (result.Id != 0 || result.Name != "运维钉钉群" || result.Secret != "fixture-secret") {
				t.Fatal("rename changed webhook identity or signing secret")
			}
		})
	}
}

func TestLegacyWebhookCustomNameIsLoaded(t *testing.T) {
	mock := mockTaskGroupDB(t)
	mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\?").WithArgs(WebhookCode).
		WillReturnRows(sqlmock.NewRows([]string{"code", "key", "value"}).
			AddRow(WebhookCode, WebhookUrlKey, "https://example.invalid/robot").
			AddRow(WebhookCode, WebhookNameKey, "运维钉钉群"))
	mock.ExpectQuery("SELECT id, `key`, value FROM `setting` WHERE code = \\? ORDER BY id").
		WithArgs(webhookEndpointCode).WillReturnRows(sqlmock.NewRows([]string{"id", "key", "value"}))
	list, err := new(Setting).WebhookEndpoints()
	if err != nil || len(list) != 1 || list[0].Name != "运维钉钉群" || list[0].Id != 0 {
		t.Fatal("persisted default webhook name was not loaded")
	}
}
