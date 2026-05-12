package promptbuilder

import "errors"

var (
	ErrBuilderNil       = errors.New("prompt builder is nil")
	ErrSceneEmpty       = errors.New("scene is empty")
	ErrPromptEngineNil  = errors.New("prompt engine is nil")
	ErrContextEngineNil = errors.New("context engine is nil")
	ErrTemplateNotFound = errors.New("template binding for scene not found")
	ErrPresetNotFound   = errors.New("scene preset not found")
)
