package skillruntime

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillSpecJSONRoundTrip(t *testing.T) {
	original := SkillSpec{
		Manifest: SkillManifest{
			Name:          "paper-reader",
			Description:   "Read papers for non-academics",
			Runtime:       "agentspec",
			Version:       "1.0.0",
			License:       "MIT",
			Compatibility: "wukong",
			Tags:          []string{"paper", "reading"},
			Metadata: map[string]string{
				"owner": "test",
			},
		},
		Instructions: "Read the paper and extract usable ideas.",
		AllowedTools: []string{"Read", "WebSearch"},
		RootDir:      "/skills/paper-reader",
		Scripts: []SkillResource{
			{Kind: "script", Name: "parse", Path: "scripts/parse.py", MIMEType: "text/x-python"},
		},
		References: []SkillResource{
			{Kind: "reference", Name: "guide", Path: "references/guide.md", Text: "notes"},
		},
		Assets: []SkillResource{
			{Kind: "asset", Name: "cover", Path: "assets/cover.png", MIMEType: "image/png"},
		},
		Metadata: map[string]any{
			"priority": float64(10),
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded SkillSpec
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Manifest.Name != original.Manifest.Name {
		t.Fatalf("Manifest.Name = %q, want %q", decoded.Manifest.Name, original.Manifest.Name)
	}
	if decoded.Instructions != original.Instructions {
		t.Fatalf("Instructions = %q, want %q", decoded.Instructions, original.Instructions)
	}
	if len(decoded.AllowedTools) != 2 || decoded.AllowedTools[1] != "WebSearch" {
		t.Fatalf("AllowedTools = %#v, want Read and WebSearch", decoded.AllowedTools)
	}
	if len(decoded.Scripts) != 1 || decoded.Scripts[0].Path != "scripts/parse.py" {
		t.Fatalf("Scripts = %#v, want parse script", decoded.Scripts)
	}
	if decoded.Metadata["priority"] != float64(10) {
		t.Fatalf("Metadata priority = %#v, want 10", decoded.Metadata["priority"])
	}
}

func TestRuntimeContextJSONRoundTrip(t *testing.T) {
	original := RuntimeContext{
		Caller: Caller{
			Type: CallerTypeAgent,
			ID:   "agent-general",
			Role: "general",
		},
		TaskID:    "task-1",
		SessionID: "session-1",
		Goal:      "answer",
		Action:    "web_search",
		Params: map[string]any{
			"query": "wukong",
		},
		Memory: map[string]any{
			"lesson": "prefer concise output",
		},
		Metadata: map[string]any{
			"source": "test",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded RuntimeContext
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Caller.Type != CallerTypeAgent || decoded.Caller.ID != original.Caller.ID {
		t.Fatalf("Caller = %#v, want %#v", decoded.Caller, original.Caller)
	}
	if decoded.Params["query"] != "wukong" {
		t.Fatalf("Params = %#v, want query", decoded.Params)
	}
}

func TestSkillRuntimePackageDoesNotImportAgent(t *testing.T) {
	root := "."
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == "agentspec" || name == "legacy" || name == "sandbox" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			if strings.Trim(spec.Path.Value, "\"") == "github.com/jiujuan/wukong/pkg/agent" {
				t.Fatalf("%s imports pkg/agent", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
}
