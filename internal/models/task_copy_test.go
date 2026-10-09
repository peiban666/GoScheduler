package models

import (
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCopyNamesAreUniqueAndFitUnicodeColumn(t *testing.T) {
	used := map[string]bool{"备份（副本）": true, "备份（副本2）": true}
	if name := copyTaskName("  备份  ", used); name != "备份（副本3）" {
		t.Fatalf("unexpected duplicate name: %s", name)
	}
	for _, original := range []string{"", strings.Repeat("中", 32), "😀" + strings.Repeat("中", 31)} {
		first := copyTaskName(original, used)
		second := copyTaskName(original, used)
		if first == second || utf8.RuneCountInString(first) > 32 || utf8.RuneCountInString(second) > 32 {
			t.Fatalf("invalid copy names: %q, %q", first, second)
		}
	}
}

func TestCopyDependenciesRemapOnlySelectedTasks(t *testing.T) {
	copies := map[int]int{1: 101, 2: 102}
	got, err := remapCopyDependencies("2, 90,1", copies)
	if err != nil || got != "102,90,101" {
		t.Fatalf("unexpected dependencies: %q, %v", got, err)
	}
	for _, value := range []string{"bad", "0", "1,", "-1"} {
		if _, err := remapCopyDependencies(value, copies); err == nil {
			t.Fatalf("accepted invalid source dependencies %q", value)
		}
	}
	if _, err := remapCopyDependencies(strings.Repeat("999999999,", 8)+"1", copies); err == nil {
		t.Fatal("accepted dependencies longer than the existing column")
	}
}

func TestCopyRejectsInvalidInputBeforeOpeningDatabase(t *testing.T) {
	for _, ids := range [][]int{nil, {}, {0}, {-1}, make([]int, MaxCopyTasks+1)} {
		if _, err := new(Task).CopyTasks(ids, "目标"); err == nil {
			t.Fatalf("accepted IDs %v", ids)
		}
	}
	if _, err := new(Task).CopyTasks([]int{1}, strings.Repeat("中", 33)); err == nil {
		t.Fatal("accepted an invalid destination")
	}
}

func copySourceRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "name", "level", "dependency_task_id", "dependency_status",
		"spec", "protocol", "command", "request_body", "http_method", "timeout", "multi",
		"retry_times", "retry_interval", "notify_status", "notify_type", "notify_receiver_id",
		"notify_keyword", "tag", "remark", "status", "created", "deleted"}).
		AddRow(1, "原任务", 1, "2,90", 2, "0 */5 * * * *", 2, "echo fixture", `{"fixture":true}`,
			2, 120, 0, 3, 15, 2, 3, "0,7", "keyword", "原分组", "备注", 1,
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), nil).
		AddRow(2, "子任务", 2, "", 1, "", 1, "https://fixture.invalid/job", "",
			1, 60, 1, 0, 0, 0, 0, "", "", "原分组", "", 1,
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), nil)
}

func expectCopySources(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `task` WHERE .*`id` IN \\(\\?,\\?\\).*ORDER BY `id` ASC.*FOR UPDATE$").
		WithArgs(1, 2, sqlmock.AnyArg()).WillReturnRows(copySourceRows())
	mock.ExpectQuery("SELECT .* FROM `task_host` WHERE .*`task_id` IN \\(\\?,\\?\\).*FOR UPDATE$").
		WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "host_id"}).
		AddRow(3, 1, 70).AddRow(4, 1, 71))
	mock.ExpectQuery("SELECT `name` FROM `task` WHERE .*").
		WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"name"}).
		AddRow("原任务").AddRow("子任务").AddRow("原任务（副本）"))
}

func expectedCloneArgs(name, group string, child bool) []driver.Value {
	if child {
		return []driver.Value{name, 2, "", 1, "", 1, "https://fixture.invalid/job", "", 1,
			60, 1, 0, 0, 0, 0, "", "", group, "", 0, sqlmock.AnyArg()}
	}
	return []driver.Value{name, 1, "", 2, "0 */5 * * * *", 2, "echo fixture", `{"fixture":true}`,
		2, 120, 0, 3, 15, 2, 3, "0,7", "keyword", group, "备注", 0, sqlmock.AnyArg()}
}

func TestCopyTransactionPreservesConfigurationAndRemapsNodesAndDependencies(t *testing.T) {
	for _, group := range []string{"目标分组", ""} {
		t.Run(group, func(t *testing.T) {
			mock := mockTaskGroupDB(t)
			expectCopySources(mock)
			if group != "" {
				mock.ExpectQuery("SELECT .* FROM `setting` WHERE .*code = \\? AND `key` = \\?.*LIMIT 1$").
					WithArgs(TaskGroupSettingCode, group).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				mock.ExpectExec("INSERT INTO `setting`").
					WithArgs(TaskGroupSettingCode, group, "").WillReturnResult(sqlmock.NewResult(8, 1))
			}
			mock.ExpectExec("INSERT INTO `task`").
				WithArgs(expectedCloneArgs("原任务（副本2）", group, false)...).
				WillReturnResult(sqlmock.NewResult(101, 1))
			mock.ExpectExec("INSERT INTO `task`").
				WithArgs(expectedCloneArgs("子任务（副本）", group, true)...).
				WillReturnResult(sqlmock.NewResult(102, 1))
			mock.ExpectExec("UPDATE `task` SET `dependency_task_id` = \\? WHERE .*`id`=\\?").
				WithArgs("102,90", 101).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO `task_host`").
				WithArgs(101, 70).WillReturnResult(sqlmock.NewResult(20, 1))
			mock.ExpectExec("INSERT INTO `task_host`").
				WithArgs(101, 71).WillReturnResult(sqlmock.NewResult(21, 1))
			mock.ExpectCommit()
			got, err := new(Task).CopyTasks([]int{1, 2, 1}, group)
			want := []CopiedTask{{SourceId: 1, Id: 101, Name: "原任务（副本2）", Status: Disabled},
				{SourceId: 2, Id: 102, Name: "子任务（副本）", Status: Disabled}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected copy result: %#v, %v", got, err)
			}
		})
	}
}

func TestCopyMissingOrDeletedSourceRollsBackWithoutCreatingAnything(t *testing.T) {
	mock := mockTaskGroupDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `task` WHERE .*FOR UPDATE$").
		WithArgs(1, 2, sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectRollback()
	if _, err := new(Task).CopyTasks([]int{1, 2}, "目标"); err == nil || !strings.Contains(err.Error(), "已不存在") {
		t.Fatalf("missing source was not rejected: %v", err)
	}
}

func TestCopyNodeFailureRollsBackAllNewTasks(t *testing.T) {
	mock := mockTaskGroupDB(t)
	expectCopySources(mock)
	mock.ExpectExec("INSERT INTO `task`").
		WithArgs(expectedCloneArgs("原任务（副本2）", "", false)...).
		WillReturnResult(sqlmock.NewResult(101, 1))
	mock.ExpectExec("INSERT INTO `task`").
		WithArgs(expectedCloneArgs("子任务（副本）", "", true)...).
		WillReturnResult(sqlmock.NewResult(102, 1))
	mock.ExpectExec("UPDATE `task` SET `dependency_task_id` = \\? WHERE .*`id`=\\?").
		WithArgs("102,90", 101).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `task_host`").
		WithArgs(101, 70).WillReturnError(errors.New("fixture node write failure"))
	mock.ExpectRollback()
	if result, err := new(Task).CopyTasks([]int{1, 2}, ""); err == nil || result != nil {
		t.Fatalf("partial copy was not rolled back: %#v, %v", result, err)
	}
}
