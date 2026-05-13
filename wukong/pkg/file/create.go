package file

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/afero"
)

func (o *Operator) CreateDir(path string, permMode ...os.FileMode) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("dir path is empty")
	}
	perm := fileMode(0o755, permMode...)
	if err := o.FS().MkdirAll(path, perm); err != nil {
		return "", err
	}
	return CleanPath(path), nil
}

func (o *Operator) CreateDirIn(parentDir, name string, permMode ...os.FileMode) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	target, err := pathInDir(parentDir, name)
	if err != nil {
		return "", err
	}
	return o.CreateDir(target, permMode...)
}

func (o *Operator) CreateFile(path string, content []byte, permMode ...os.FileMode) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("file path is empty")
	}
	if err := o.EnsureParent(path); err != nil {
		return "", err
	}
	perm := fileMode(0o644, permMode...)
	if err := afero.WriteFile(o.FS(), path, content, perm); err != nil {
		return "", err
	}
	return CleanPath(path), nil
}

func (o *Operator) CreateTextFile(path, content string, permMode ...os.FileMode) (string, error) {
	return o.CreateFile(path, []byte(content), permMode...)
}

func (o *Operator) CreateEmptyFile(path string, permMode ...os.FileMode) (string, error) {
	return o.CreateFile(path, nil, permMode...)
}

func (o *Operator) CreateFileInDir(dir, name string, content []byte, permMode ...os.FileMode) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	target, err := pathInDir(dir, name)
	if err != nil {
		return "", err
	}
	return o.CreateFile(target, content, permMode...)
}

func (o *Operator) CreateTextFileInDir(dir, name, content string, permMode ...os.FileMode) (string, error) {
	return o.CreateFileInDir(dir, name, []byte(content), permMode...)
}
