package skills

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jiujuan/wukong/pkg/sandbox"
)

type PackageManifest struct {
	PackageName string             `json:"package_name,omitempty"`
	Version     string             `json:"version,omitempty"`
	Homepage    string             `json:"homepage,omitempty"`
	Runtime     string             `json:"runtime,omitempty"`
	Entry       string             `json:"entry,omitempty"`
	Tools       []string           `json:"tools,omitempty"`
	Permissions PackagePermissions `json:"permissions,omitempty"`
}

type PackagePermissions struct {
	Tools      []string `json:"tools,omitempty"`
	Network    bool     `json:"network,omitempty"`
	FileRoots  []string `json:"file_roots,omitempty"`
	AllowShell bool     `json:"allow_shell,omitempty"`
}

func loadPackageManifest(dir string) (PackageManifest, string, error) {
	manifestPath := filepath.Join(dir, "wukong.skill.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return PackageManifest{}, "", nil
		}
		return PackageManifest{}, "", err
	}
	var manifest PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PackageManifest{}, "", fmt.Errorf("parse manifest %s: %w", manifestPath, err)
	}
	manifest.Tools = normalizeStringSlice(append(manifest.Tools, manifest.Permissions.Tools...))
	manifest.PackageName = strings.TrimSpace(manifest.PackageName)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Homepage = strings.TrimSpace(manifest.Homepage)
	manifest.Runtime = strings.TrimSpace(manifest.Runtime)
	manifest.Entry = strings.TrimSpace(manifest.Entry)
	return manifest, manifestPath, nil
}

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func resolveSkillResources(rootDir string) ([]string, []string, map[string]any, error) {
	rootAbs, err := filepath.Abs(strings.TrimSpace(rootDir))
	if err != nil {
		return nil, nil, nil, err
	}
	rootAbs = filepath.Clean(rootAbs)

	references, err := walkSkillResourceDir(rootAbs, "references")
	if err != nil {
		return nil, nil, nil, err
	}
	assets, err := walkSkillResourceDir(rootAbs, "assets")
	if err != nil {
		return nil, nil, nil, err
	}

	metadata := map[string]any{
		"resource_root":    rootAbs,
		"references":       append([]string(nil), references...),
		"assets":           append([]string(nil), assets...),
		"references_count": len(references),
		"assets_count":     len(assets),
	}
	return references, assets, metadata, nil
}

func walkSkillResourceDir(rootAbs, name string) ([]string, error) {
	dir := filepath.Join(rootAbs, name)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	items := make([]string, 0)
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		abs, err := resolveSkillResourcePath(rootAbs, path)
		if err != nil {
			return err
		}
		items = append(items, abs)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(items)
	return items, nil
}

func resolveSkillResourcePath(rootAbs, candidate string) (string, error) {
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if !sandbox.WithinAllowedRoots(abs, []string{rootAbs}) {
		return "", fmt.Errorf("skill resource path is outside root: %s", abs)
	}
	return abs, nil
}
