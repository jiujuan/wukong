package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

func exists(fs afero.Fs, path string) (bool, error) {
	if fs == nil {
		return false, nil
	}
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if isNotExist(err) {
		return false, nil
	}
	return false, err
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, afero.ErrFileNotFound)
}

func fileMode(defaultMode os.FileMode, modes ...os.FileMode) os.FileMode {
	if len(modes) > 0 && modes[0] > 0 {
		return modes[0]
	}
	return defaultMode
}

func pathInDir(dir, name string) (string, error) {
	dir = CleanPath(dir)
	name = strings.TrimSpace(name)
	if dir == "" || dir == "." {
		return "", fmt.Errorf("dir is empty")
	}
	if name == "" {
		return "", fmt.Errorf("name is empty")
	}
	target := filepath.Clean(filepath.Join(dir, name))
	if !isSubPath(dir, target) {
		return "", fmt.Errorf("path escapes dir: %s", name)
	}
	return target, nil
}
