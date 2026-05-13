package skills

import (
	"bytes"
	"encoding/json"
)

func changedWithCurrent(current map[string]*Skill, loaded map[string]*Skill) bool {
	if len(current) != len(loaded) {
		return true
	}
	for k, v := range current {
		other, ok := loaded[k]
		if !ok {
			return true
		}
		a, _ := json.Marshal(v)
		b, _ := json.Marshal(other)
		if !bytes.Equal(a, b) {
			return true
		}
	}
	return false
}

func cloneSkill(src *Skill) *Skill {
	if src == nil {
		return nil
	}
	dst := *src
	if src.Params != nil {
		dst.Params = append([]Param(nil), src.Params...)
	}
	if src.Tools != nil {
		dst.Tools = append([]string(nil), src.Tools...)
	}
	if src.References != nil {
		dst.References = append([]string(nil), src.References...)
	}
	if src.Assets != nil {
		dst.Assets = append([]string(nil), src.Assets...)
	}
	if src.Metadata != nil {
		dst.Metadata = make(map[string]any, len(src.Metadata))
		for key, value := range src.Metadata {
			dst.Metadata[key] = value
		}
	}
	return &dst
}
