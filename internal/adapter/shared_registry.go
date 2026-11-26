package adapter

import (
	"github.com/clang-engineer/themeset/internal/adapter/atuin"
	"github.com/clang-engineer/themeset/internal/adapter/bat"
	"github.com/clang-engineer/themeset/internal/adapter/btop"
	"github.com/clang-engineer/themeset/internal/adapter/delta"
	"github.com/clang-engineer/themeset/internal/adapter/eza"
	"github.com/clang-engineer/themeset/internal/adapter/fzf"
	"github.com/clang-engineer/themeset/internal/adapter/ghostty"
	"github.com/clang-engineer/themeset/internal/adapter/harlequin"
	"github.com/clang-engineer/themeset/internal/adapter/lazydocker"
	"github.com/clang-engineer/themeset/internal/adapter/lazygit"
	"github.com/clang-engineer/themeset/internal/adapter/neovim"
	"github.com/clang-engineer/themeset/internal/adapter/starship"
	"github.com/clang-engineer/themeset/internal/adapter/tmux"
	"github.com/clang-engineer/themeset/internal/adapter/yazi"
	"github.com/clang-engineer/themeset/internal/model"
)

func All() []model.Adapter {
	return []model.Adapter{
		ghostty.Adapter(), neovim.Adapter(), tmux.Adapter(), bat.Adapter(),
		delta.Adapter(), harlequin.Adapter(), starship.Adapter(), fzf.Adapter(),
		yazi.Adapter(), atuin.Adapter(), lazygit.Adapter(), lazydocker.Adapter(),
		btop.Adapter(), eza.Adapter(),
	}
}
