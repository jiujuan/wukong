package file

import (
	"fmt"

	"github.com/spf13/afero"
)

type Operator struct {
	fs afero.Fs
}

func New(fs afero.Fs) *Operator {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return &Operator{fs: fs}
}

func NewOS() *Operator {
	return New(afero.NewOsFs())
}

func (o *Operator) FS() afero.Fs {
	if o == nil || o.fs == nil {
		return afero.NewOsFs()
	}
	return o.fs
}

func (o *Operator) ensureReady() error {
	if o == nil {
		return fmt.Errorf("file operator is nil")
	}
	if o.fs == nil {
		return fmt.Errorf("file operator fs is nil")
	}
	return nil
}
