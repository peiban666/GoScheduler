package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const MaxCopyTasks = 1000

type CopiedTask struct {
	SourceId int    `json:"source_id"`
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Status   Status `json:"status"`
}

func copyTaskName(original string, used map[string]bool) string {
	base := []rune(strings.TrimSpace(original))
	if len(base) == 0 {
		base = []rune("任务")
	}
	for number := 1; ; number++ {
		suffix := "（副本）"
		if number > 1 {
			suffix = fmt.Sprintf("（副本%d）", number)
		}
		length := 32 - len([]rune(suffix))
		if length > len(base) {
			length = len(base)
		}
		name := string(base[:length]) + suffix
		if !used[name] {
			used[name] = true
			return name
		}
	}
}

func remapCopyDependencies(value string, copies map[int]int) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	parts := strings.Split(value, ",")
	for i, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			return "", fmt.Errorf("原任务依赖配置无效，请先检查子任务")
		}
		if copied, exists := copies[id]; exists {
			parts[i] = strconv.Itoa(copied)
		} else {
			parts[i] = strings.TrimSpace(part)
		}
	}
	result := strings.Join(parts, ",")
	if len(result) > 64 {
		return "", fmt.Errorf("副本的依赖任务编号超过 64 个字符，请减少关联子任务")
	}
	return result, nil
}

// CopyTasks creates disabled tasks and their node associations atomically.
// Only dependencies within the selected set are remapped to new IDs; references
// outside the selection remain unchanged. Existing tasks and timers are untouched.
func (task *Task) CopyTasks(ids []int, name string) ([]CopiedTask, error) {
	taskGroupMu.Lock()
	defer taskGroupMu.Unlock()
	name, err := NormalizeTaskGroupName(name)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 || len(ids) > MaxCopyTasks {
		return nil, fmt.Errorf("请选择 1–%d 个任务", MaxCopyTasks)
	}
	unique := make([]int, 0, len(ids))
	seen := make(map[int]bool)
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("请选择有效的任务")
		}
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	session := Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return nil, err
	}
	defer session.Rollback()

	sources := make([]Task, 0)
	if err := session.In("id", unique).Asc("id").ForUpdate().Find(&sources); err != nil {
		return nil, err
	}
	if len(sources) != len(unique) {
		return nil, fmt.Errorf("部分任务已不存在，请刷新后重试")
	}
	sourceHosts := make([]TaskHost, 0)
	if err := session.In("task_id", unique).ForUpdate().Find(&sourceHosts); err != nil {
		return nil, err
	}
	names := make([]Task, 0)
	if err := session.Cols("name").Find(&names); err != nil {
		return nil, err
	}
	used := make(map[string]bool, len(names)+len(unique))
	for _, item := range names {
		used[item.Name] = true
	}
	if name != "" {
		exists, err := session.Where("code = ? AND "+Db.Quote("key")+" = ?", TaskGroupSettingCode, name).Exist(new(Setting))
		if err != nil {
			return nil, err
		}
		if !exists {
			if _, err := session.Insert(&Setting{Code: TaskGroupSettingCode, Key: name}); err != nil {
				return nil, err
			}
		}
	}

	byID := make(map[int]Task, len(sources))
	for _, source := range sources {
		byID[source.Id] = source
	}
	copies := make(map[int]int, len(unique))
	result := make([]CopiedTask, 0, len(unique))
	for _, id := range unique {
		clone := byID[id]
		clone.Id = 0
		clone.Name = copyTaskName(clone.Name, used)
		clone.Tag = name
		clone.Status = Disabled
		clone.Created = time.Time{}
		clone.Deleted = time.Time{}
		clone.NextRunTime = time.Time{}
		clone.Hosts = nil
		clone.BaseModel = BaseModel{}
		// Insert all IDs before remapping dependencies, including forward references.
		clone.DependencyTaskId = ""
		if _, err := session.Insert(&clone); err != nil {
			return nil, err
		}
		copies[id] = clone.Id
		result = append(result, CopiedTask{SourceId: id, Id: clone.Id, Name: clone.Name, Status: Disabled})
	}
	for _, id := range unique {
		dependencies, err := remapCopyDependencies(byID[id].DependencyTaskId, copies)
		if err != nil {
			return nil, err
		}
		if dependencies != "" {
			if _, err := session.Table(new(Task)).ID(copies[id]).
				Update(CommonMap{"dependency_task_id": dependencies}); err != nil {
				return nil, err
			}
		}
	}
	for _, source := range sourceHosts {
		clone := TaskHost{TaskId: copies[source.TaskId], HostId: source.HostId}
		if _, err := session.Insert(&clone); err != nil {
			return nil, err
		}
	}
	if err := session.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
