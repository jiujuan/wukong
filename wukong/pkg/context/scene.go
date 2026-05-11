package context

type SceneConfig struct {
	Name     string
	Sources  []string
	Policies []string
	Options  map[string]any
}

func cloneScene(src SceneConfig) SceneConfig {
	dst := src
	if src.Sources != nil {
		dst.Sources = append([]string(nil), src.Sources...)
	}
	if src.Policies != nil {
		dst.Policies = append([]string(nil), src.Policies...)
	}
	if src.Options != nil {
		dst.Options = make(map[string]any, len(src.Options))
		for k, v := range src.Options {
			dst.Options[k] = v
		}
	}
	return dst
}
