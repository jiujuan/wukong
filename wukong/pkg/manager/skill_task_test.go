package manager

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jiujuan/wukong/pkg/queue"
	"github.com/jiujuan/wukong/pkg/skills"
)

func TestPlanTaskUsesThirdPartySkillExecution(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "local", "echo_skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	scriptName := "run.sh"
	scriptBody := "#!/usr/bin/env bash\nprintf \"skill=%s\\n\" \"$SKILL_NAME\"\n"
	if runtime.GOOS == "windows" {
		scriptName = "run.ps1"
		scriptBody = `Write-Output "skill=$env:SKILL_NAME"`
	}
	if err := os.WriteFile(filepath.Join(skillDir, scriptName), []byte(scriptBody), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	if err := os.WriteFile(skillFile, []byte(strings.Join([]string{
		"# Skill: echo_skill",
		"## Description",
		"third party skill",
		"## Tools",
		"- llm_chat",
		"## Execute",
		"- " + scriptName,
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}

	registry := skills.New(skills.WithRootDir(root))
	if err := registry.Start(context.Background()); err != nil {
		t.Fatalf("start registry failed: %v", err)
	}
	defer registry.Stop()

	mgr := NewManager(nil)
	mgr.SetSkillRegistry(registry)
	mgr.SetPlanner(NewTplPlanner())

	task := &Task{
		TaskID:    "task-skill-1",
		SkillName: "echo_skill",
		Params:    map[string]any{"query": "golang"},
		Status:    "PENDING",
	}
	if err := mgr.planTask(&queue.Task{TaskID: task.TaskID, Data: task}); err != nil {
		t.Fatalf("planTask failed: %v", err)
	}

	subtasks, err := mgr.GetSubTasks(context.Background(), task.TaskID)
	if err != nil {
		t.Fatalf("GetSubTasks failed: %v", err)
	}
	if len(subtasks) != 1 {
		t.Fatalf("subtask count = %d, want 1", len(subtasks))
	}
	sub := subtasks[0]
	if sub.Action != "echo_skill" {
		t.Fatalf("subtask action = %q, want echo_skill", sub.Action)
	}
	if got := sub.Params["execution_type"]; got != "third_party_skill" {
		t.Fatalf("execution_type = %#v, want third_party_skill", got)
	}
	if got := sub.Params["skill_source_type"]; got != string(skills.SourceLocal) {
		t.Fatalf("skill_source_type = %#v, want %q", got, skills.SourceLocal)
	}
}

func TestAggregateResultsMarksThirdPartySkillExecution(t *testing.T) {
	mgr := NewManager(nil)
	result := mgr.aggregateResults([]*SubTask{
		{
			SubTaskID: "sub-1",
			TaskID:    "task-1",
			Action:    "echo_skill",
			Status:    "SUCCESS",
			Result: map[string]any{
				"execution_type": "third_party_skill",
				"skill_name":     "echo_skill",
				"stdout":         "skill=echo_skill",
				"stderr":         "",
				"exit_code":      0,
				"output_dir":     "storage/output_data/echo_skill",
			},
		},
	})
	if result["_execution_type"] != "third_party_skill" {
		t.Fatalf("_execution_type = %#v, want third_party_skill", result["_execution_type"])
	}
	exec, ok := result["_execution"].(map[string]any)
	if !ok {
		t.Fatalf("_execution missing: %#v", result)
	}
	if exec["skill_name"] != "echo_skill" {
		t.Fatalf("execution skill_name = %#v, want echo_skill", exec["skill_name"])
	}
}
