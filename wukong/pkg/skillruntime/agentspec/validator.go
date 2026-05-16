package agentspec

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jiujuan/wukong/pkg/skillruntime"
)

var (
	// ErrFrontmatterNotFound is returned when SKILL.md does not start with YAML frontmatter.
	ErrFrontmatterNotFound = errors.New("agentspec frontmatter not found")
	// ErrFrontmatterEndNotFound is returned when SKILL.md frontmatter has no closing marker.
	ErrFrontmatterEndNotFound = errors.New("agentspec frontmatter end marker not found")
	// ErrNameRequired is returned when frontmatter omits name.
	ErrNameRequired = errors.New("agentspec skill name is required")
	// ErrDescriptionRequired is returned when frontmatter omits description.
	ErrDescriptionRequired = errors.New("agentspec skill description is required")
)

// Validator validates parsed Agent Skills Spec models.
type Validator struct{}

// Validate checks the required Agent Skills Spec fields.
func (Validator) Validate(spec *skillruntime.SkillSpec) error {
	if spec == nil {
		return fmt.Errorf("agentspec skill spec is nil")
	}
	if strings.TrimSpace(spec.Manifest.Name) == "" {
		return ErrNameRequired
	}
	if strings.TrimSpace(spec.Manifest.Description) == "" {
		return ErrDescriptionRequired
	}
	return nil
}
