package context

import (
	"fmt"
	"strings"
)

func normalizeBlocks(defaultSource string, blocks []ContextBlock) []ContextBlock {
	out := make([]ContextBlock, 0, len(blocks))
	for _, block := range blocks {
		block.Name = strings.TrimSpace(block.Name)
		block.Type = strings.TrimSpace(block.Type)
		block.Source = strings.TrimSpace(block.Source)
		block.Content = strings.TrimSpace(block.Content)
		if block.Content == "" {
			continue
		}
		if block.Source == "" {
			block.Source = defaultSource
		}
		out = append(out, block)
	}
	return out
}

func formatBlockText(block ContextBlock) string {
	if block.Name == "" {
		return block.Content
	}
	return fmt.Sprintf("[%s]\n%s", block.Name, block.Content)
}
