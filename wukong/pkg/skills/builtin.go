package skills

func defaultBuiltins() map[string]*Skill {
	return map[string]*Skill{
		"chat": {
			SkillName:   "chat",
			Description: "基础对话技能",
			Version:     "1.0.0",
			Enabled:     true,
			Tools:       []string{"llm_chat"},
			Memory: MemoryConfig{
				MemoryType:     "working",
				WindowSize:     5,
				CompressSwitch: true,
			},
		},
		"web_search": {
			SkillName:   "web_search",
			Description: "联网搜索技能",
			Version:     "1.0.0",
			Enabled:     true,
			Tools:       []string{"web_search", "http_request", "llm_chat"},
			Memory: MemoryConfig{
				MemoryType:     "working",
				WindowSize:     10,
				CompressSwitch: true,
			},
		},
		"report_gen": {
			SkillName:   "report_gen",
			Description: "报告生成技能",
			Version:     "1.0.0",
			Enabled:     true,
			Tools:       []string{"llm_chat", "file_write"},
			Memory: MemoryConfig{
				MemoryType:     "long_term",
				WindowSize:     20,
				CompressSwitch: true,
			},
		},
	}
}
