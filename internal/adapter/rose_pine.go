package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

// Exact upstream Rosé Pine (main variant) application ports. All downloads
// happen on apply; resolve and dry-run never access the network or asset cache.
type rosePineAsset struct {
	url, sha256, licenseURL, licenseSHA256 string
}

func rosePineRecipe(id string) (rosePineAsset, bool) {
	var repo, revision, file, digest, license string
	switch id {
	case "starship":
		repo, revision, file = "starship", "0edc3c781f219565453bbb8e8e7af56ebc2a0d8a", "rose-pine.toml"
		digest, license = "d272b7811403ba81fe8c0bca59b334e2b5545ed14d460bc1f3a8ca86ebe7afd8", "b2ec9e48689252be5807d35d80689bbc40481c54d1f228c94dde8ffad1bdc824"
	case "fzf":
		repo, revision, file = "fzf", "9af300f871df52331c362ca58c73535f69b6fb03", "dist/rose-pine.sh"
		digest, license = "10b467c088f37746751c3ae66f80aa7fc247f91e3c41165609e34def4542ce9c", "b2ec9e48689252be5807d35d80689bbc40481c54d1f228c94dde8ffad1bdc824"
	case "lazygit":
		repo, revision, file = "lazygit", "acc968a6090bd190ab0c49fa36847cb931a39a52", "themes/rose-pine.yml"
		digest, license = "c633466ec21b92a6e0a2762c75ecb76608e09184d6a05210b523ca19e1d8a0fa", "0547abf2995a6f34c1d15450840adf6470ddae855008c576df8f631f2da506b4"
	case "atuin":
		repo, revision, file = "atuin", "4680e8a3eda07476bfb73ee040df005d604e65ed", "dist/rose-pine.toml"
		digest, license = "06e599ab0f172f0f0ae645ff63feb17cc7636d54292d2de51f765f9eae61b5bd", "638a6cb33a2b304656cd8e47e53d5eba925adbe79bac9eb89dae099ae073263c"
	case "eza":
		repo, revision, file = "eza", "ffd796b8265cc6395231f4ad663e6749f34fc897", "dist/rose-pine.yml"
		digest, license = "192d052032663e6983c2756242a3451d6986591211aceaa1e7beb9ab6f75c180", "638a6cb33a2b304656cd8e47e53d5eba925adbe79bac9eb89dae099ae073263c"
	case "btop":
		repo, revision, file = "btop", "c27e5d48e44e8bd115a2838ecf8cf1f3ea39475e", "rose-pine.theme"
		digest, license = "088675387b0b3665f0424ac6b250748decd8f2387a0acf821bc8f47a80cfd2c6", "fb2535bcb42729f0547691eea7feee12d6870d184d704a7a929513973974d7bd"
	default:
		return rosePineAsset{}, false
	}
	base := "https://raw.githubusercontent.com/rose-pine/" + repo + "/" + revision + "/"
	return rosePineAsset{base + file, digest, base + "LICENSE", license}, true
}

func filesResolveRosePinePort(id, name string, _ model.Environment) (model.Port, bool, error) {
	if name != "rose-pine" {
		return model.Port{}, false, nil
	}
	recipe, ok := rosePineRecipe(id)
	if !ok {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "official", Name: name, Source: recipe.url}, true, nil
}

func filesAcquireRosePinePort(id string, port model.Port, env model.Environment) ([]byte, error) {
	recipe, ok := rosePineRecipe(id)
	if !ok || port.Kind != "official" || port.Name != "rose-pine" || port.Source != recipe.url || port.File != "" {
		return nil, fmt.Errorf("unregistered Rosé Pine %s port", id)
	}
	data, err := filesAcquireAsset(env, recipe.url, recipe.sha256)
	if err != nil {
		return nil, fmt.Errorf("acquire Rosé Pine %s theme: %w", id, err)
	}
	switch id {
	case "starship":
		data, err = filesPaletteFamilyTOML(data, "rose-pine")
	case "fzf":
		data, err = rosePineFZFOptions(data)
	case "lazygit":
		var config map[string]any
		config, err = filesYAML(data)
		if err == nil {
			gui, _ := config["gui"].(map[string]any)
			theme, _ := gui["theme"].(map[string]any)
			if len(theme) == 0 {
				err = errors.New("missing lazygit gui.theme")
			}
			// Preserve the complete official gui.theme; filesApplyLazy imports
			// only that table, leaving unrelated GUI preferences untouched.
		}
	case "atuin":
		var name string
		name, err = filesAtuinTheme(data)
		if err == nil && name != port.Name {
			err = fmt.Errorf("unexpected Atuin theme name %q", name)
		}
	case "eza":
		err = rosePineValidateEza(data)
	case "btop":
		err = filesBtopTheme(data)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid Rosé Pine %s theme: %w", id, err)
	}
	if _, err := filesAcquireAsset(env, recipe.licenseURL, recipe.licenseSHA256); err != nil {
		return nil, fmt.Errorf("acquire Rosé Pine %s license: %w", id, err)
	}
	return data, nil
}

// The upstream Rosé Pine distribution has no \"colourful\" toggle; validate its
// native eza style tables without imposing Gruvbox's additional setting.
func rosePineValidateEza(data []byte) error {
	config, err := filesYAML(data)
	if err != nil {
		return err
	}
	for _, section := range []string{"filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type"} {
		styles, ok := config[section].(map[string]any)
		if !ok || len(styles) == 0 {
			return fmt.Errorf("Rosé Pine eza requires native %s styles", section)
		}
		for key, style := range styles {
			if err := filesValidateEzaFamilyStyle(section+"."+key, style); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"punctuation", "date", "inode", "blocks", "header", "octal", "flags", "symlink_path", "control_char", "broken_symlink", "broken_path_overlay"} {
		if err := filesValidateEzaFamilyStyle(key, config[key]); err != nil {
			return err
		}
	}
	return nil
}

// Discard the shell export, retaining only the author's exact fzf color options.
func rosePineFZFOptions(script []byte) ([]byte, error) {
	lines := strings.Split(strings.TrimSpace(string(script)), "\n")
	if len(lines) < 2 || lines[0] != `export FZF_DEFAULT_OPTS="` || !strings.HasSuffix(strings.TrimSpace(lines[len(lines)-1]), `"`) {
		return nil, errors.New("unexpected Rosé Pine fzf shell wrapper")
	}
	lines[len(lines)-1] = strings.TrimSuffix(strings.TrimSpace(lines[len(lines)-1]), `"`)
	var result strings.Builder
	for _, line := range lines[1:] {
		option := strings.TrimSpace(line)
		if !strings.HasPrefix(option, "--color=") {
			return nil, fmt.Errorf("unexpected Rosé Pine fzf option %q", option)
		}
		result.WriteString(option)
		result.WriteByte('\n')
	}
	data := []byte(result.String())
	if err := filesFZFOptions(data); err != nil {
		return nil, err
	}
	return data, nil
}

const rosePineYaziBase = "https://raw.githubusercontent.com/rose-pine/yazi/c89d745573d4fcfe0550fe6646f9f9ab1c0e51db/"
const rosePineYaziSource = "https://github.com/rose-pine/yazi/tree/c89d745573d4fcfe0550fe6646f9f9ab1c0e51db/flavors/rose-pine.yazi"

var rosePineYaziAssets = []struct{ path, digest string }{
	{"flavors/rose-pine.yazi/flavor.toml", "68632b9dc12328d59b437e9a45cfbdabb32129118b93ad6ca03a1188883a0a54"},
	{"flavors/rose-pine.yazi/tmtheme.xml", "c5d16ccb41bbdd0145388d8443ff9213249e30e20bde233773ab22db3678cb43"},
	{"LICENSE", "8f807f6c12ba9a17cc10d92f9ecac4c709795b6dff3f93543983e814dbec3d8b"},
}

func filesResolveRosePineYazi(name string, env model.Environment) (model.Port, bool, error) {
	if name != "rose-pine" {
		return model.Port{}, false, nil
	}
	port := model.Port{Kind: "official", Name: name, Source: rosePineYaziSource}
	installed, err := filesYaziFlavorInstalled(name, env)
	if err != nil || !installed {
		return port, err == nil, err
	}
	root := filesYaziFlavorRoot(name, env)
	for _, asset := range rosePineYaziAssets {
		data, err := filesRead(filepath.Join(root, filepath.Base(asset.path)), true)
		if err != nil {
			return model.Port{}, false, err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != asset.digest {
			port.Kind = "native"
			port.File = filepath.Join(root, "flavor.toml")
			break
		}
	}
	return port, true, nil
}

func filesInstallRosePineYazi(env model.Environment) error {
	installed, err := filesYaziFlavorInstalled("rose-pine", env)
	if err != nil || installed {
		return err
	}
	root := filesYaziFlavorRoot("rose-pine", env)
	info, err := os.Lstat(root)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if exists && !info.IsDir() {
		return fmt.Errorf("refusing to replace non-directory Rosé Pine Yazi flavor %s", root)
	}
	var content [3][]byte
	for i, asset := range rosePineYaziAssets {
		file := filepath.Join(root, filepath.Base(asset.path))
		content[i], err = filesRead(file, true)
		if err != nil {
			return err
		}
		if content[i] != nil {
			sum := sha256.Sum256(content[i])
			if hex.EncodeToString(sum[:]) != asset.digest {
				return fmt.Errorf("incomplete Rosé Pine Yazi flavor conflicts with %s", file)
			}
		} else {
			content[i], err = filesAcquireAsset(env, rosePineYaziBase+asset.path, asset.digest)
			if err != nil {
				return fmt.Errorf("acquire Rosé Pine Yazi %s: %w", asset.path, err)
			}
		}
	}
	if exists {
		for i, asset := range rosePineYaziAssets {
			if err := filesAtomic(filepath.Join(root, filepath.Base(asset.path)), content[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(root), ".themeset-rose-pine-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for i, asset := range rosePineYaziAssets {
		if err := filesAtomic(filepath.Join(stage, filepath.Base(asset.path)), content[i]); err != nil {
			return err
		}
	}
	return os.Rename(stage, root)
}

func filesApplyRosePineYazi(port model.Port, env model.Environment) error {
	if port.Name != "rose-pine" || port.Kind != "official" && port.Kind != "native" || port.Kind == "official" && port.Source != rosePineYaziSource || port.Kind == "native" && port.File != filepath.Join(filesYaziFlavorRoot("rose-pine", env), "flavor.toml") {
		return errors.New("unregistered Rosé Pine Yazi flavor")
	}
	if port.Kind == "native" {
		installed, err := filesYaziFlavorInstalled("rose-pine", env)
		if err != nil || !installed {
			return fmt.Errorf("Rosé Pine native Yazi flavor unavailable: %v", err)
		}
	}
	return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if port.Kind == "official" {
			if err := filesInstallRosePineYazi(env); err != nil {
				return err
			}
		}
		flavor, err := filesTable(config, "flavor")
		if err != nil {
			return err
		}
		flavor["dark"] = "rose-pine"
		flavor["light"] = "rose-pine"
		delete(flavor, "use")
		return nil
	})
}
