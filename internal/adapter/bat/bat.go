package bat

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "bat", Binary: "bat", Resolve: nativeResolveBat, Apply: nativeApplyBat}
}

func nativeResolveBat(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	source := spec.NativeSource
	if source == "" {
		source = shared.BatThemesSource
	}
	port, ok, err := shared.ListedCandidates("bat", spec.Native, source, env, "--list-themes", "--color=never")
	if ok || err != nil {
		return port, ok, err
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

var nativeBatTheme = regexp.MustCompile(`^\s*--theme(?:\s*=|\s+|$)`)

func nativeParseBat(data []byte) (string, error) {
	name := ""
	for _, line := range strings.Split(string(data), "\n") {
		if !nativeBatTheme.MatchString(line) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "--theme"))
		value = strings.TrimSpace(strings.TrimPrefix(value, "="))
		if strings.HasPrefix(value, "\"") {
			parsed, err := strconv.Unquote(value)
			if err != nil {
				return "", err
			}
			value = parsed
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = strings.Trim(value, "'")
		}
		if value == "" || strings.ContainsAny(value, "\r\n\x00") {
			return "", fmt.Errorf("invalid bat theme selection")
		}
		name = value
	}
	if name == "" {
		return "", fmt.Errorf("custom bat config contains no --theme selection")
	}
	return name, nil
}

func nativeApplyBat(port model.Port, env model.Environment) error {
	name, err := shared.Selection(port, env, nativeParseBat)
	if err != nil {
		return err
	}
	path := shared.NativeExpand(env.Paths["bat"], env)
	if path == "" {
		out, err := shared.Command(env, "bat", "--config-file")
		if err != nil {
			return err
		}
		path = strings.TrimSpace(string(out))
		if !filepath.IsAbs(path) {
			return fmt.Errorf("bat returned a non-absolute native config path %q", path)
		}
	}
	data, err := shared.Read(path, true)
	if err != nil {
		return err
	}
	return shared.Atomic(path, shared.ReplaceLines(data, func(s string) bool { return nativeBatTheme.MatchString(s) }, "--theme="+strconv.Quote(name)))
}
