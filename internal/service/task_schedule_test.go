package service

import (
	"testing"
	"time"

	"github.com/gaggad/goscheduler/internal/models"
	"github.com/jakecoffman/cron"
)

func TestOneCronEntryPerTask(t *testing.T) {
	old := serviceCron
	serviceCron = cron.New()
	defer func() { serviceCron = old }()
	task := models.Task{Id: 42, Level: models.TaskLevelParent, Protocol: models.TaskHTTP, Spec: "0 0 3,20 * * *\n0 30 7 * * *"}
	ServiceTask.Add(task)
	entries := serviceCron.Entries()
	if len(entries) != 1 || entries[0].Name != "42" {
		t.Fatalf("expected one entry named 42, got %v", entries)
	}
	after := time.Date(2026, 10, 7, 6, 0, 0, 0, time.UTC)
	if next := entries[0].Schedule.Next(after); next.Hour() != 7 || next.Minute() != 30 {
		t.Fatalf("wrong next execution: %s", next)
	}
	task.Spec = "0 50 10 * * *"
	ServiceTask.RemoveAndAdd(task)
	entries = serviceCron.Entries()
	if len(entries) != 1 {
		t.Fatalf("edit leaked old schedules: %v", entries)
	}
	if next := entries[0].Schedule.Next(after); next.Hour() != 10 || next.Minute() != 50 {
		t.Fatalf("edit retained old execution: %s", next)
	}
	ServiceTask.Remove(task.Id)
	if len(serviceCron.Entries()) != 0 {
		t.Fatal("remove left a schedule behind")
	}
}

func TestMinuteIntervalIsOneReplaceableCronEntry(t *testing.T) {
	old := serviceCron
	serviceCron = cron.New()
	defer func() { serviceCron = old }()
	task := models.Task{Id: 43, Level: models.TaskLevelParent, Protocol: models.TaskHTTP, Spec: "@every 7m"}
	ServiceTask.Add(task)
	entries := serviceCron.Entries()
	if len(entries) != 1 || entries[0].Name != "43" {
		t.Fatalf("expected one interval entry, got %v", entries)
	}
	after := time.Date(2026, 10, 9, 12, 3, 17, 100000000, time.UTC)
	if next := entries[0].Schedule.Next(after); !next.Equal(time.Date(2026, 10, 9, 12, 7, 0, 0, time.UTC)) {
		t.Fatalf("wrong interval execution: %s", next)
	}
	task.Spec = "@every 30m"
	ServiceTask.RemoveAndAdd(task)
	entries = serviceCron.Entries()
	if len(entries) != 1 {
		t.Fatalf("interval edit leaked entries: %v", entries)
	}
	if next := entries[0].Schedule.Next(after); !next.Equal(time.Date(2026, 10, 9, 12, 30, 0, 0, time.UTC)) {
		t.Fatalf("interval edit retained old minute steps: %s", next)
	}
	task.Spec = "0 0 */2 * * *"
	ServiceTask.RemoveAndAdd(task)
	entries = serviceCron.Entries()
	if len(entries) != 1 {
		t.Fatalf("hour interval edit leaked entries: %v", entries)
	}
	if next := entries[0].Schedule.Next(after); !next.Equal(time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)) {
		t.Fatalf("hour interval was not anchored at midnight: %s", next)
	}
}
