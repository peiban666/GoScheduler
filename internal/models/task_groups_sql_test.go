package models

import (
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-xorm/xorm"
)

// Exercise the actual xorm-generated SQL without opening a production database.
func mockTaskGroupDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := xorm.NewEngine("mysql", "fixture:fixture@tcp(127.0.0.1:3306)/fixture")
	if err != nil {
		t.Fatal(err)
	}
	engine.DB().DB.Close()
	engine.DB().DB = sqlDB
	original := Db
	Db = engine
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		Db = original
		engine.Close()
	})
	return mock
}

func expectRenameSource(mock sqlmock.Sqlmock, name string, taskIDs []int, explicit bool) {
	mock.ExpectBegin()
	tasks := sqlmock.NewRows([]string{"id"})
	for _, id := range taskIDs {
		tasks.AddRow(id)
	}
	mock.ExpectQuery("SELECT .* FROM `task` WHERE .*TRIM\\(tag\\) = \\?.*FOR UPDATE$").
		WithArgs(name, sqlmock.AnyArg()).WillReturnRows(tasks)
	definitions := sqlmock.NewRows([]string{"id", "code", "key", "value"})
	if explicit {
		definitions.AddRow(7, TaskGroupSettingCode, name, "")
	}
	mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\? AND `key` = \\?.*FOR UPDATE$").
		WithArgs(TaskGroupSettingCode, name).WillReturnRows(definitions)
}

func expectRenameDestination(mock sqlmock.Sqlmock, name string, explicit, implicit bool) {
	definitions := sqlmock.NewRows([]string{"id"})
	if explicit {
		definitions.AddRow(8)
	}
	mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\? AND `key` = \\?.*LIMIT 1$").
		WithArgs(TaskGroupSettingCode, name).WillReturnRows(definitions)
	if explicit {
		return
	}
	tasks := sqlmock.NewRows([]string{"id"})
	if implicit {
		tasks.AddRow(9)
	}
	mock.ExpectQuery("SELECT .* FROM `task` WHERE .*TRIM\\(tag\\) = \\?.*LIMIT 1$").
		WithArgs(name, sqlmock.AnyArg()).WillReturnRows(tasks)
}

func TestRenameGroupSQLPreservesTaskColumnsAndScopesSettings(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "implicit", true: "explicit"}[explicit], func(t *testing.T) {
			mock := mockTaskGroupDB(t)
			expectRenameSource(mock, "old", []int{1, 26}, explicit)
			expectRenameDestination(mock, "new", false, false)
			// Updating only tag must not rewrite spec/status or reset timers.
			mock.ExpectExec("UPDATE `task` SET `tag` = \\? WHERE .*`id` IN \\(\\?,\\?\\)").
				WithArgs("new", 1, 26).WillReturnResult(sqlmock.NewResult(0, 2))
			if explicit {
				mock.ExpectExec("UPDATE `setting` SET `key` = \\? WHERE .*code = \\? AND `key` = \\?").
					WithArgs("new", TaskGroupSettingCode, "old").WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec("INSERT INTO `setting`").
					WithArgs(TaskGroupSettingCode, "new", "").WillReturnResult(sqlmock.NewResult(7, 1))
			}
			mock.ExpectCommit()
			if err := new(Task).RenameGroup(" old ", " new "); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRenameEmptyGroupSQL(t *testing.T) {
	mock := mockTaskGroupDB(t)
	expectRenameSource(mock, "empty", nil, true)
	expectRenameDestination(mock, "renamed", false, false)
	mock.ExpectExec("UPDATE `setting` SET `key` = \\? WHERE .*code = \\? AND `key` = \\?").
		WithArgs("renamed", TaskGroupSettingCode, "empty").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := new(Task).RenameGroup("empty", "renamed"); err != nil {
		t.Fatal(err)
	}
}

func TestRenameGroupSQLDuplicateOrMissingRollsBack(t *testing.T) {
	for _, mode := range []string{"explicit-target", "implicit-target", "missing-source"} {
		t.Run(mode, func(t *testing.T) {
			mock := mockTaskGroupDB(t)
			if mode == "missing-source" {
				expectRenameSource(mock, "old", nil, false)
			} else {
				expectRenameSource(mock, "old", []int{1}, false)
				expectRenameDestination(mock, "new", mode == "explicit-target", mode == "implicit-target")
			}
			mock.ExpectRollback()
			err := new(Task).RenameGroup("old", "new")
			if err == nil || (!strings.Contains(err.Error(), "已存在") && !strings.Contains(err.Error(), "已不存在")) {
				t.Fatalf("expected a conflict or missing source, got %v", err)
			}
		})
	}
}

func TestRenameGroupSQLSameNameDoesNotWrite(t *testing.T) {
	mock := mockTaskGroupDB(t)
	expectRenameSource(mock, "old", []int{1}, true)
	mock.ExpectCommit()
	if err := new(Task).RenameGroup("old", " old "); err != nil {
		t.Fatal(err)
	}
}

func TestRenameGroupSQLMetadataFailureRollsBackTaskUpdate(t *testing.T) {
	mock := mockTaskGroupDB(t)
	expectRenameSource(mock, "old", []int{1}, true)
	expectRenameDestination(mock, "new", false, false)
	mock.ExpectExec("UPDATE `task` SET `tag` = \\? WHERE .*`id` IN \\(\\?\\)").
		WithArgs("new", 1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `setting` SET `key` = \\? WHERE .*code = \\? AND `key` = \\?").
		WithArgs("new", TaskGroupSettingCode, "old").WillReturnError(errors.New("fixture write failure"))
	mock.ExpectRollback()
	if err := new(Task).RenameGroup("old", "new"); err == nil || !strings.Contains(err.Error(), "fixture write failure") {
		t.Fatalf("expected metadata write failure, got %v", err)
	}
}

func TestDeleteEmptyGroupSQLScopesCodeAndKey(t *testing.T) {
	mock := mockTaskGroupDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `task` WHERE .*TRIM\\(tag\\) = \\?.*FOR UPDATE$").
		WithArgs("empty", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\? AND `key` = \\?.*LIMIT 1$").
		WithArgs(TaskGroupSettingCode, "empty").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec("DELETE FROM `setting` WHERE .*code = \\? AND `key` = \\?").
		WithArgs(TaskGroupSettingCode, "empty").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if _, err := new(Task).DeleteGroup("empty", false, 0); err != nil {
		t.Fatal(err)
	}
}
