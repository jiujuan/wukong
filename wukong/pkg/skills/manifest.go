package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
