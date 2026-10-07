package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestTaskGroupsSpanPagesAndDeduplicateHostJoins(t *testing.T) {
	tasks := []Task{{Id: 1}, {Id: 2, Tag: "未分组"}, {Id: 3, Tag: "备份"}, {Id: 3, Tag: "备份"}}
	for id := 4; id <= 54; id++ {
		tasks = append(tasks, Task{Id: id, Tag: "备份"})
	}
	got := summarizeTaskGroups(tasks)
	want := []TaskGroup{{Name: "", Total: 1}, {Name: "备份", Total: 52}, {Name: "未分组", Total: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
	if got := summarizeTaskGroups(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty groups should serialize as [], got %#v", got)
	}
}

func TestNormalizeTaskGroupName(t *testing.T) {
	for _, name := range []string{"", "  备份  ", "__proto__", strings.Repeat("中", 32)} {
		got, err := NormalizeTaskGroupName(name)
		if err != nil || got != strings.TrimSpace(name) {
			t.Fatalf("name %q: got %q, %v", name, got, err)
		}
	}
	for _, name := range []string{strings.Repeat("中", 33), "a\nb", "a\tb", "a\x00b"} {
		if _, err := NormalizeTaskGroupName(name); err == nil {
			t.Fatalf("accepted invalid group %q", name)
		}
	}
}

func TestParseTaskGroupIDs(t *testing.T) {
	got, err := ParseTaskGroupIDs("1, 2,1,3")
	if err != nil || !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("IDs = %v, %v", got, err)
	}
	for _, value := range []string{"", "0", "-1", "1,", "1,abc", "1.5", "99999999999999999999999999"} {
		if _, err := ParseTaskGroupIDs(value); err == nil {
			t.Fatalf("accepted invalid IDs %q", value)
		}
	}
}

func TestExplicitEmptyGroupsRespectSearchFilters(t *testing.T) {
	names := []string{"空组", "备份", "空组", ""}
	tasks := []Task{{Id: 1, Tag: "备份"}}
	got := mergeTaskGroups(tasks, names, CommonMap{})
	want := []TaskGroup{{Name: "备份", Total: 1}, {Name: "空组", Total: 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
	for _, params := range []CommonMap{
		{"Ungrouped": true}, {"Tag": "其他组"}, {"Name": "task"},
		{"Id": 1}, {"HostId": 2}, {"Protocol": 1}, {"Status": 0}, {"Status": 1},
	} {
		if groups := mergeTaskGroups(nil, names, params); len(groups) != 0 {
			t.Fatalf("empty groups ignore filter %#v: %#v", params, groups)
		}
	}
	got = mergeTaskGroups(nil, names, CommonMap{"Tag": "空组", "Status": -1})
	if len(got) != 1 || got[0].Name != "空组" || got[0].Total != 0 {
		t.Fatalf("empty named group must remain selectable: %#v", got)
	}
}

func TestGroupDeletionRejectsDefaultGroupAndUnconfirmedCount(t *testing.T) {
	if _, err := new(Task).DeleteGroup("", false, 0); err == nil {
		t.Fatal("built-in ungrouped should not be deleted")
	}
	if _, err := new(Task).DeleteGroup("group", true, -1); err == nil {
		t.Fatal("deletion must require an explicitly confirmed task count")
	}
	if err := new(Task).CreateGroup(""); err == nil {
		t.Fatal("empty group names should be rejected before accessing DB")
	}
}

func TestGroupRenameRejectsInvalidNamesBeforeDatabaseAccess(t *testing.T) {
	for _, names := range [][2]string{
		{"", "new"}, {"   ", "new"}, {"old", ""}, {"old", "   "},
		{"old", strings.Repeat("中", 33)}, {"old", "a\nb"},
		{strings.Repeat("中", 33), "new"},
	} {
		if err := new(Task).RenameGroup(names[0], names[1]); err == nil {
			t.Fatalf("accepted invalid rename: %#v", names)
		}
	}
}
