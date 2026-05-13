package file

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/afero"
)

func (o *Operator) WriteFile(path string, data []byte, permMode ...os.FileMode) error {
	if err := o.ensureReady(); err != nil {
		return err
	}
	if err := o.EnsureParent(path); err != nil {
		return err
	}
	perm := fileMode(0o644, permMode...)
	return afero.WriteFile(o.FS(), path, data, perm)
}

func (o *Operator) WriteString(path string, content string, permMode ...os.FileMode) error {
	return o.WriteFile(path, []byte(content), permMode...)
}

func (o *Operator) AppendFile(path string, data []byte, permMode ...os.FileMode) error {
	if err := o.ensureReady(); err != nil {
		return err
	}
	if err := o.EnsureParent(path); err != nil {
		return err
	}
	perm := fileMode(0o644, permMode...)
	f, err := o.FS().OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

func (o *Operator) WriteInDir(dir, name, content string, permMode ...os.FileMode) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("dir is empty")
	}
	if err := o.EnsureDir(dir); err != nil {
		return "", err
	}
	target, err := pathInDir(dir, name)
	if err != nil {
		return "", err
	}
	if err := o.WriteString(target, content, permMode...); err != nil {
		return "", err
	}
	return target, nil
}
