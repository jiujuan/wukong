package file

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
)

type ScanOptions struct {
	Recursive bool
	FilesOnly bool
	DirsOnly  bool
	Exts      []string
	Prefix    string
	Contains  string
}

type EntryInfo struct {
	Path  string
	Name  string
	IsDir bool
	Size  int64
}

func (o *Operator) ScanDir(dir string, opts ScanOptions) ([]EntryInfo, error) {
	if err := o.ensureReady(); err != nil {
		return nil, err
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("dir is empty")
	}
	info, err := o.FS().Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	allowedExts := normalizeExts(opts.Exts)
	prefix := strings.TrimSpace(opts.Prefix)
	contains := strings.TrimSpace(opts.Contains)
	var entries []EntryInfo
	walkFn := func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		if !opts.Recursive && filepath.Dir(path) != filepath.Clean(dir) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if opts.FilesOnly && info.IsDir() {
			return nil
		}
		if opts.DirsOnly && !info.IsDir() {
			return nil
		}
		name := info.Name()
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			return nil
		}
		if contains != "" && !strings.Contains(name, contains) {
			return nil
		}
		if info.IsDir() && len(allowedExts) > 0 {
			return nil
		}
		if !info.IsDir() && len(allowedExts) > 0 {
			if _, ok := allowedExts[strings.ToLower(filepath.Ext(name))]; !ok {
				return nil
			}
		}
		entries = append(entries, EntryInfo{
			Path:  path,
			Name:  name,
			IsDir: info.IsDir(),
			Size:  info.Size(),
		})
		return nil
	}
	if opts.Recursive {
		if err := afero.Walk(o.FS(), dir, walkFn); err != nil {
			return nil, err
		}
	} else {
		list, err := afero.ReadDir(o.FS(), dir)
		if err != nil {
			return nil, err
		}
		for _, info := range list {
			path := filepath.Join(dir, info.Name())
			if err := walkFn(path, info, nil); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func normalizeExts(exts []string) map[string]struct{} {
	if len(exts) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		result[ext] = struct{}{}
	}
	return result
}
