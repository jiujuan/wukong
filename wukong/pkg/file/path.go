package file

import (
	"fmt"
	"path/filepath"
	"strings"
)

func CleanPath(path string) string {
	return filepath.Clean(strings.TrimSpace(path))
}

func Join(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	if len(cleaned) == 0 {
		return "."
	}
	return filepath.Join(cleaned...)
}

func AbsPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	return filepath.Abs(path)
}

func (o *Operator) Exists(path string) (bool, error) {
	if err := o.ensureReady(); err != nil {
		return false, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return false, nil
	}
	ok, err := exists(o.FS(), path)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (o *Operator) IsFile(path string) (bool, error) {
	if err := o.ensureReady(); err != nil {
		return false, err
	}
	info, err := o.FS().Stat(path)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

func (o *Operator) IsDir(path string) (bool, error) {
	if err := o.ensureReady(); err != nil {
		return false, err
	}
	info, err := o.FS().Stat(path)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func (o *Operator) EnsureDir(dir string) error {
	if err := o.ensureReady(); err != nil {
		return err
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("dir is empty")
	}
	return o.FS().MkdirAll(dir, 0o755)
}

func (o *Operator) EnsureParent(path string) error {
	if err := o.ensureReady(); err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return o.EnsureDir(dir)
}

func (o *Operator) ConfirmDir(dir string) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	dir = CleanPath(dir)
	if dir == "" || dir == "." {
		return "", fmt.Errorf("dir is empty")
	}
	ok, err := o.IsDir(dir)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("dir does not exist: %s", dir)
	}
	return dir, nil
}

func (o *Operator) ConfirmFile(path string) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	path = CleanPath(path)
	if path == "" || path == "." {
		return "", fmt.Errorf("file path is empty")
	}
	ok, err := o.IsFile(path)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("file does not exist: %s", path)
	}
	return path, nil
}

func (o *Operator) FilePath(baseDir, name string) (string, error) {
	return o.pathUnderBase(baseDir, name, false)
}

func (o *Operator) DirPath(baseDir, name string) (string, error) {
	return o.pathUnderBase(baseDir, name, true)
}

func (o *Operator) pathUnderBase(baseDir, name string, wantDir bool) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	target, err := pathInDir(baseDir, name)
	if err != nil {
		return "", err
	}
	if wantDir {
		return o.ConfirmDir(target)
	}
	return o.ConfirmFile(target)
}

func isSubPath(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	if target == base {
		return true
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
