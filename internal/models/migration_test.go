package models

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gaggad/goscheduler/internal/modules/logger"
)

const columnExistsQuery = "SELECT `COLUMN_NAME` FROM `INFORMATION_SCHEMA`.`COLUMNS` WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ? AND `COLUMN_NAME` = ?"

func TestUpgrade154ChecksExistingSchemaBeforeAddingColumn(t *testing.T) {
	logger.InitLogger()
	for _, test := range []struct {
		name   string
		prefix string
		exists bool
		fail   bool
	}{
		{name: "upstream-current-schema", exists: true},
		{name: "older-schema"},
		{name: "prefixed-current-schema", prefix: "cron_", exists: true},
		{name: "prefixed-older-schema", prefix: "cron_"},
		{name: "inspection-failure", fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			mock := mockTaskGroupDB(t)
			originalPrefix := TablePrefix
			TablePrefix = test.prefix
			t.Cleanup(func() { TablePrefix = originalPrefix })
			query := mock.ExpectQuery(regexp.QuoteMeta(columnExistsQuery)).
				WithArgs("fixture", test.prefix+"task", "request_body")
			if test.fail {
				query.WillReturnError(errors.New("fixture schema inspection failure"))
			} else {
				rows := sqlmock.NewRows([]string{"COLUMN_NAME"})
				if test.exists {
					rows.AddRow("request_body")
				}
				query.WillReturnRows(rows)
				if !test.exists {
					mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE `" + test.prefix + "task` ADD COLUMN request_body TEXT")).
						WillReturnResult(sqlmock.NewResult(0, 0))
				}
			}
			session := Db.NewSession()
			defer session.Close()
			err := new(Migration).upgradeFor154(session)
			if (err != nil) != test.fail {
				t.Fatalf("unexpected migration result: %v", err)
			}
		})
	}
}

func TestUpgradeFromUpstream150KeepsExistingRequestBody(t *testing.T) {
	logger.InitLogger()
	mock := mockTaskGroupDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(columnExistsQuery)).
		WithArgs("fixture", "task", "request_body").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("request_body"))
	// No ALTER, UPDATE, INSERT or DELETE is expected for the current schema.
	mock.ExpectCommit()
	new(Migration).Upgrade(150)
}
