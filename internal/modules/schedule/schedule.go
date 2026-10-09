// Package schedule combines cron expressions into one task schedule.
package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jakecoffman/cron"
)

// MaxSpecLength matches the existing task and task_log spec columns.
const MaxSpecLength = 64

// Normalize accepts one expression, or expressions separated by newlines/semicolons.
// Keeping the existing spec field avoids a database migration.
func Normalize(spec string) (string, error) {
	spec = strings.ReplaceAll(spec, "\r\n", "\n")
	spec = strings.ReplaceAll(spec, ";", "\n")
	seen := make(map[string]bool)
	expressions := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(spec), "\n") {
		expression := strings.Join(strings.Fields(line), " ")
		if expression == "" {
			continue
		}
		if !seen[expression] {
			seen[expression] = true
			expressions = append(expressions, expression)
		}
	}
	normalized := strings.Join(expressions, "\n")
	if normalized == "" {
		return "", fmt.Errorf("请选择执行时间或输入 crontab 表达式")
	}
	if len(normalized) > MaxSpecLength {
		return "", fmt.Errorf("调度表达式超过 %d 个字符，请减少时间点", MaxSpecLength)
	}
	return normalized, nil
}

// Parse preserves legacy cron expressions and descriptors, including @every.
func Parse(spec string) (result cron.Schedule, err error) {
	normalized, err := Normalize(spec)
	if err != nil {
		return nil, err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("crontab 表达式解析失败: %v", recovered)
		}
	}()
	expressions := strings.Split(normalized, "\n")
	schedules := make([]cron.Schedule, 0, len(expressions))
	for _, expression := range expressions {
		parsed := cron.Parse(expression)
		if _, interval := parsed.(cron.ConstantDelaySchedule); interval && len(expressions) > 1 {
			return nil, fmt.Errorf("@every 间隔规则请单独使用，多个时间点请使用六字段 Cron 表达式")
		}
		// Earlier visual pickers saved @every Nm. Existing minute rules up to
		// one hour now use the same minute-00 calendar steps as the new picker,
		// without rewriting task data or anchoring execution to save/restart time.
		// Longer legacy durations and advanced second-based rules keep their meaning.
		fields := strings.Fields(expression)
		if len(fields) == 2 && fields[0] == "@every" && strings.HasSuffix(fields[1], "m") {
			minutes, parseErr := strconv.ParseUint(strings.TrimSuffix(fields[1], "m"), 10, 64)
			if delay, ok := parsed.(cron.ConstantDelaySchedule); ok && parseErr == nil && minutes > 0 {
				if minutes <= 60 {
					parsed = cron.Parse(fmt.Sprintf("0 */%d * * * *", minutes))
				} else {
					parsed = MinuteIntervalSchedule{Delay: delay.Delay}
				}
			}
		}
		schedules = append(schedules, parsed)
	}
	if len(schedules) == 1 {
		return schedules[0], nil
	}
	return combinedSchedule{schedules: schedules}, nil
}

type MinuteIntervalSchedule struct {
	Delay time.Duration
}

func (schedule MinuteIntervalSchedule) Next(after time.Time) time.Time {
	return after.Truncate(time.Minute).Add(schedule.Delay)
}

type combinedSchedule struct {
	schedules []cron.Schedule
}

// Next returns the earliest time across all expressions. Overlapping expressions
// still produce a single cron entry and therefore a single execution per tick.
func (s combinedSchedule) Next(after time.Time) time.Time {
	var earliest time.Time
	for _, schedule := range s.schedules {
		next := schedule.Next(after)
		if !next.IsZero() && (earliest.IsZero() || next.Before(earliest)) {
			earliest = next
		}
	}
	return earliest
}
