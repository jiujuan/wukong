package file

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/afero"
)

func (o *Operator) ReadFile(path string) ([]byte, error) {
	if err := o.ensureReady(); err != nil {
		return nil, err
	}
	return afero.ReadFile(o.FS(), path)
}

func (o *Operator) ReadString(path string) (string, error) {
	data, err := o.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (o *Operator) ReadRange(path string, offset, length int64) ([]byte, error) {
	if err := o.ensureReady(); err != nil {
		return nil, err
	}
	f, err := o.FS().Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	if length <= 0 {
		return io.ReadAll(f)
	}
	buf := make([]byte, length)
	n, err := io.ReadFull(f, buf)
	switch {
	case err == nil:
		return buf[:n], nil
	case err == io.EOF || err == io.ErrUnexpectedEOF:
		return buf[:n], nil
	default:
		return nil, err
	}
}

func (o *Operator) ReadHead(path string, length int64) (string, error) {
	data, err := o.ReadRange(path, 0, length)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (o *Operator) ReadTail(path string, length int64) (string, error) {
	if err := o.ensureReady(); err != nil {
		return "", err
	}
	if length <= 0 {
		return "", fmt.Errorf("length must be positive")
	}
	data, err := o.ReadFile(path)
	if err != nil {
		return "", err
	}
	if int64(len(data)) <= length {
		return string(data), nil
	}
	return string(data[len(data)-int(length):]), nil
}

func (o *Operator) ReadLines(path string, start, count int) ([]string, error) {
	if err := o.ensureReady(); err != nil {
		return nil, err
	}
	text, err := o.ReadString(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(text, "\n")
	if start < 0 {
		start = 0
	}
	if start >= len(lines) {
		return []string{}, nil
	}
	end := len(lines)
	if count > 0 && start+count < end {
		end = start + count
	}
	return append([]string(nil), lines[start:end]...), nil
}
