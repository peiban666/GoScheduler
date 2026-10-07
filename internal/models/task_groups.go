package models

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const MaxTaskGroupNameLength = 32
const TaskGroupSettingCode = "task_group"

var taskGroupMu sync.Mutex

// Membership uses tag; empty group definitions use the existing setting table.
type TaskGroup struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
}

func NormalizeTaskGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > MaxTaskGroupNameLength {
		return "", fmt.Errorf("分组名称最多 %d 个字符", MaxTaskGroupNameLength)
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return "", fmt.Errorf("分组名称请使用单行文本")
		}
	}
	return name, nil
}

func ParseTaskGroupIDs(value string) ([]int, error) {
	ids := make([]int, 0)
	seen := make(map[int]bool)
	for _, text := range strings.Split(value, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("请选择有效的任务")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > 1000 {
		return nil, fmt.Errorf("一次最多移动 1000 个任务")
	}
	return ids, nil
}

func summarizeTaskGroups(tasks []Task) []TaskGroup {
	counts := make(map[string]int)
	seen := make(map[int]bool)
	for _, task := range tasks {
		if seen[task.Id] {
			continue
		}
		seen[task.Id] = true
		counts[strings.TrimSpace(task.Tag)]++
	}
	groups := make([]TaskGroup, 0, len(counts))
	for name, total := range counts {
		groups = append(groups, TaskGroup{Name: name, Total: total})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

// Summaries span all matching tasks, never just the first task page.
func (task *Task) Groups(params CommonMap) ([]TaskGroup, error) {
	explicit, err := task.explicitGroupNames()
	if err != nil {
		return nil, err
	}
	tasks := make([]Task, 0)
	session := Db.Alias("t").Join("LEFT", taskHostTableName(), "t.id = th.task_id")
	task.parseWhere(session, params)
	err = session.GroupBy("t.id").Cols("t.id", "t.tag").Find(&tasks)
	if err != nil {
		return nil, err
	}
	return mergeTaskGroups(tasks, explicit, params), nil
}

func mergeTaskGroups(tasks []Task, explicit []string, params CommonMap) []TaskGroup {
	groups := summarizeTaskGroups(tasks)
	counts := make(map[string]int, len(groups))
	for _, group := range groups {
		counts[group.Name] = group.Total
	}
	// Explicit groups remain visible after their last task is moved or deleted.
	for _, name := range explicit {
		if name == "" || !includeEmptyTaskGroup(name, params) {
			continue
		}
		if _, exists := counts[name]; !exists {
			groups = append(groups, TaskGroup{Name: name})
			counts[name] = 0
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func includeEmptyTaskGroup(name string, params CommonMap) bool {
	if ungrouped, _ := params["Ungrouped"].(bool); ungrouped {
		return false
	}
	if tag, _ := params["Tag"].(string); tag != "" && tag != name {
		return false
	}
	if search, _ := params["Name"].(string); search != "" {
		return false
	}
	for _, key := range []string{"Id", "HostId", "Protocol"} {
		if value, _ := params[key].(int); value > 0 {
			return false
		}
	}
	if status, ok := params["Status"].(int); ok && status >= 0 {
		return false
	}
	return true
}

func (task *Task) explicitGroupNames() ([]string, error) {
	settings := make([]Setting, 0)
	err := Db.Where("code = ?", TaskGroupSettingCode).Cols("key").Find(&settings)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(settings))
	seen := make(map[string]bool)
	for _, setting := range settings {
		name := strings.TrimSpace(setting.Key)
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (task *Task) CreateGroup(name string) error {
	taskGroupMu.Lock()
	defer taskGroupMu.Unlock()
	name, err := NormalizeTaskGroupName(name)
	if err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("分组名称不能为空")
	}
	exists, err := Db.Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, name).Exist(new(Setting))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("分组已存在")
	}
	exists, err = Db.Where("TRIM(tag) = ?", name).Exist(new(Task))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("分组已存在")
	}
	_, err = Db.Insert(&Setting{Code: TaskGroupSettingCode, Key: name})
	return err
}

// RenameGroup changes only group metadata and tags; running timers are untouched.
func (task *Task) RenameGroup(oldName, newName string) error {
	taskGroupMu.Lock()
	defer taskGroupMu.Unlock()
	oldName, err := NormalizeTaskGroupName(oldName)
	if err != nil {
		return err
	}
	if oldName == "" {
		return fmt.Errorf("未分组是固定分组")
	}
	newName, err = NormalizeTaskGroupName(newName)
	if err != nil {
		return err
	}
	if newName == "" {
		return fmt.Errorf("分组名称不能为空")
	}
	session := Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return err
	}
	defer session.Rollback()

	tasks := make([]Task, 0)
	if err := session.Where("TRIM(tag) = ?", oldName).Cols("id").ForUpdate().Find(&tasks); err != nil {
		return err
	}
	definitions := make([]Setting, 0)
	if err := session.Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, oldName).
		ForUpdate().Find(&definitions); err != nil {
		return err
	}
	if len(definitions) == 0 && len(tasks) == 0 {
		return fmt.Errorf("分组已不存在，请刷新后重试")
	}
	if oldName == newName {
		return session.Commit()
	}
	exists, err := session.Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, newName).Exist(new(Setting))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("分组名称已存在，请使用其他名称")
	}
	exists, err = session.Where("TRIM(tag) = ?", newName).Exist(new(Task))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("分组名称已存在，请使用其他名称")
	}
	if len(tasks) > 0 {
		ids := make([]int, 0, len(tasks))
		for _, item := range tasks {
			ids = append(ids, item.Id)
		}
		if _, err := session.Table(new(Task)).In("id", ids).Update(CommonMap{"tag": newName}); err != nil {
			return err
		}
	}
	if len(definitions) > 0 {
		if _, err := session.Table(new(Setting)).
			Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, oldName).
			Update(CommonMap{"key": newName}); err != nil {
			return err
		}
	} else if _, err := session.Insert(&Setting{Code: TaskGroupSettingCode, Key: newName}); err != nil {
		return err
	}
	return session.Commit()
}

// DeleteGroup removes the group definition. Its tasks are either ungrouped or
// deleted according to deleteTasks. The returned IDs let the service unregister
// deleted parent tasks from the in-memory scheduler.
func (task *Task) DeleteGroup(name string, deleteTasks bool, expectedCount int) ([]int, error) {
	taskGroupMu.Lock()
	defer taskGroupMu.Unlock()
	name, err := NormalizeTaskGroupName(name)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("未分组不能删除")
	}
	if expectedCount < 0 {
		return nil, fmt.Errorf("请确认组内任务数量")
	}
	session := Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return nil, err
	}
	defer session.Rollback()

	tasks := make([]Task, 0)
	if err := session.Where("TRIM(tag) = ?", name).Cols("id").ForUpdate().Find(&tasks); err != nil {
		return nil, err
	}
	if len(tasks) != expectedCount {
		return nil, fmt.Errorf("组内任务数量已变化，请重新打开删除窗口确认")
	}
	exists, err := session.Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, name).Exist(new(Setting))
	if err != nil {
		return nil, err
	}
	if !exists && len(tasks) == 0 {
		return nil, fmt.Errorf("分组已不存在，请刷新后重试")
	}
	ids := make([]int, 0, len(tasks))
	for _, item := range tasks {
		ids = append(ids, item.Id)
	}
	if len(ids) > 0 {
		if deleteTasks {
			if _, err := session.In("id", ids).Delete(new(Task)); err != nil {
				return nil, err
			}
			if _, err := session.In("task_id", ids).Delete(new(TaskHost)); err != nil {
				return nil, err
			}
		} else if _, err := session.Table(new(Task)).In("id", ids).Update(CommonMap{"tag": ""}); err != nil {
			return nil, err
		}
	}
	if _, err := session.Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, name).Delete(new(Setting)); err != nil {
		return nil, err
	}
	if err := session.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}

// AssignGroup updates only tags, preserving status, spec and running timers.
func (task *Task) AssignGroup(ids []int, name string) error {
	taskGroupMu.Lock()
	defer taskGroupMu.Unlock()
	var err error
	name, err = NormalizeTaskGroupName(name)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("请选择任务")
	}
	session := Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return err
	}
	defer session.Rollback()
	tasks := make([]Task, 0)
	if err := session.In("id", ids).Cols("id").ForUpdate().Find(&tasks); err != nil {
		return err
	}
	if len(tasks) != len(ids) {
		return fmt.Errorf("部分任务已不存在，请刷新后重试")
	}
	if name != "" {
		exists, err := session.Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, name).Exist(new(Setting))
		if err != nil {
			return err
		}
		if !exists {
			if _, err := session.Insert(&Setting{Code: TaskGroupSettingCode, Key: name}); err != nil {
				return err
			}
		}
	}
	if _, err := session.Table(new(Task)).In("id", ids).Update(CommonMap{"tag": name}); err != nil {
		return err
	}
	return session.Commit()
}
