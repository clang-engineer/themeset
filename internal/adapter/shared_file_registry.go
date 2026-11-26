package adapter

import "github.com/clang-engineer/themeset/internal/model"

const filesMoonflySource = "https://raw.githubusercontent.com/bluz71/vim-moonfly-colors/master/extras/"

func fileAdapters() []model.Adapter {
	return []model.Adapter{
		{ID: "starship", Binary: "starship", Resolve: filesResolveStarship, Apply: filesApplyStarship},
		{ID: "fzf", Binary: "fzf", Resolve: filesResolveFZF, Apply: filesApplyFZF},
		{ID: "yazi", Binary: "yazi", Resolve: filesResolveYazi, Apply: filesApplyYazi},
		{ID: "atuin", Binary: "atuin", Resolve: filesResolveAtuin, Apply: filesApplyAtuin},
		{ID: "lazygit", Binary: "lazygit", Resolve: filesResolveLazy("lazygit"), Apply: filesApplyLazy("lazygit")},
		{ID: "lazydocker", Binary: "lazydocker", Resolve: filesResolveLazy("lazydocker"), Apply: filesApplyLazy("lazydocker")},
		{ID: "btop", Binary: "btop", Resolve: filesResolveBtop, Apply: filesApplyBtop},
		{ID: "eza", Binary: "eza", Resolve: filesResolveEza, Apply: filesApplyEza},
	}
}
