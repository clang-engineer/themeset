package app

import (
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

// DefaultPort returns the bundled tool declaration. A user custom port replaces
// it for the corresponding theme and tool.
func DefaultPort(theme, tool string) model.ThemePort {
	return catalog.Port(theme, tool)
}
