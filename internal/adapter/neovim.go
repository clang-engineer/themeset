package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

type nativeNeovimProvider struct{ Name, Source string }

var nativeNeovimProviders = map[string]nativeNeovimProvider{
	"dracula":                   {"dracula", "https://github.com/dracula/vim"},
	"catppuccin-mocha":          {"catppuccin-mocha", "https://github.com/catppuccin/nvim"},
	"nord":                      {"nord", "https://github.com/nordtheme/vim"},
	"gruvbox":                   {"gruvbox", "https://github.com/ellisonleao/gruvbox.nvim"},
	"gruvbox-dark":              {"gruvbox", "https://github.com/ellisonleao/gruvbox.nvim"},
	"tokyonight":                {"tokyonight-night", "https://github.com/folke/tokyonight.nvim"},
	"tokyo-night":               {"tokyonight-night", "https://github.com/folke/tokyonight.nvim"},
	"rose-pine":                 {"rose-pine-main", "https://github.com/rose-pine/neovim"},
	"kanagawa":                  {"kanagawa-wave", "https://github.com/rebelot/kanagawa.nvim"},
	"everforest":                {"everforest", "https://github.com/sainnhe/everforest"},
	"onedark":                   {"onedark", "https://github.com/navarasu/onedark.nvim"},
	"github-dark":               {"github_dark", "https://github.com/projekt0n/github-nvim-theme"},
	"github-dark-high-contrast": {"github_dark_high_contrast", "https://github.com/projekt0n/github-nvim-theme"},
	"moonfly":                   {"moonfly", "https://github.com/bluz71/vim-moonfly-colors"},
	"vscode-dark":               {"vscode", "https://github.com/Mofiqul/vscode.nvim"},
	"sonokai":                   {"sonokai", "https://github.com/sainnhe/sonokai"},
	"cyberdream":                {"cyberdream", "https://github.com/scottmckendry/cyberdream.nvim"},
	"modus-vivendi":             {"modus_vivendi", "https://github.com/miikanissi/modus-themes.nvim"},
}

// Resolve uses on-disk runtime evidence, not a headless launch of the user's
// init.lua (which can have arbitrary side effects). No plugin is installed.
func nativeResolveNeovim(theme string, env model.Environment) (model.Port, bool, error) {
	provider, ok := nativeNeovimProviders[nativeThemeKey(theme)]
	if !ok {
		return model.Port{}, false, nil
	}
	root, err := nativeNeovimRuntime(provider.Name, env)
	if err != nil {
		return model.Port{}, false, err
	}
	if root == "" {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "native", Name: provider.Name, Source: provider.Source}, true, nil
}

func nativeNeovimRuntime(name string, env model.Environment) (string, error) {
	if strings.ContainsAny(name, "/\\\r\n\x00") || name == "" {
		return "", fmt.Errorf("invalid Neovim colorscheme name")
	}
	config := filepath.Join(nativeConfigHome(env), "nvim")
	data := filepath.Join(env.Home, ".local", "share", "nvim")
	roots := []string{config, filepath.Join(data, "site")}
	patterns := []string{filepath.Join(data, "lazy", "*"), filepath.Join(data, "site", "pack", "*", "start", "*"), filepath.Join(data, "site", "pack", "*", "opt", "*"), filepath.Join(config, "pack", "*", "start", "*"), filepath.Join(config, "pack", "*", "opt", "*")}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", err
		}
		roots = append(roots, matches...)
	}
	for _, root := range roots {
		for _, extension := range []string{".lua", ".vim"} {
			file := filepath.Join(root, "colors", name+extension)
			info, err := os.Stat(file)
			if err == nil && info.Mode().IsRegular() {
				return root, nil
			}
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}
		}
	}
	return "", nil
}

func nativeLuaString(value string) string {
	// Emit a Lua short string, including decimal escapes for control bytes.
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 32 {
				b.WriteString(fmt.Sprintf("\\%03d", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func nativeApplyNeovim(port model.Port, env model.Environment) error {
	var apply string
	if port.File != "" {
		source := nativeExpand(port.File, env)
		if !filepath.IsAbs(source) {
			return fmt.Errorf("Neovim theme file must be absolute")
		}
		ext := filepath.Ext(source)
		if ext != ".vim" && ext != ".lua" {
			return fmt.Errorf("Neovim custom file must be an existing .vim or .lua theme script")
		}
		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Neovim theme must be a regular file")
		}
		apply = "vim.cmd('source ' .. vim.fn.fnameescape(" + nativeLuaString(source) + "))"
	} else {
		root, err := nativeNeovimRuntime(port.Name, env)
		if err != nil {
			return err
		}
		if root == "" {
			return fmt.Errorf("Neovim colorscheme %q is not installed in supported runtime locations", port.Name)
		}
		apply = "vim.opt.runtimepath:prepend(" + nativeLuaString(root) + ")\n    vim.cmd('colorscheme ' .. vim.fn.fnameescape(" + nativeLuaString(port.Name) + "))"
	}
	path := nativePath("neovim", env, filepath.Join(nativeConfigHome(env), "nvim", "plugin", "themeset.lua"))
	if port.File != "" && filepath.Clean(nativeExpand(port.File, env)) == filepath.Clean(path) {
		return fmt.Errorf("Neovim theme script cannot be the managed destination plugin")
	}
	existing, err := nativeRead(path)
	if err != nil {
		return err
	}
	const marker = "-- Managed by themeset: selects an existing installed colorscheme.\n"
	if len(existing) > 0 && !strings.HasPrefix(string(existing), marker) {
		return fmt.Errorf("refusing to replace non-themeset Neovim plugin %s", path)
	}
	script := marker + `local function apply()
  local ok, err = pcall(function()
    ` + apply + `
  end)
  if not ok then
    vim.schedule(function()
      vim.notify('themeset: ' .. tostring(err), vim.log.levels.ERROR)
    end)
  end
end
if vim.v.vim_did_enter == 1 then
  apply()
else
  vim.api.nvim_create_autocmd('VimEnter', { once = true, callback = function()
    vim.schedule(apply)
  end })
end
`
	return nativeWrite(path, []byte(script))
}
