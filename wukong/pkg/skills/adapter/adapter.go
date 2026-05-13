package adapter

import "github.com/jiujuan/wukong/pkg/skills"

type Adapter interface {
	Match(path string, content []byte) bool
	Parse(path string) (*skills.Skill, error)
}
