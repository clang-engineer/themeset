package adapter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

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
	name, err := nativeSelection(port, env, nativeParseBat)
	if err != nil {
		return err
	}
	path := nativeExpand(env.Paths["bat"], env)
	if path == "" {
		out, err := nativeCommand(env, "bat", "--config-file")
		if err != nil {
			return err
		}
		path = strings.TrimSpace(string(out))
		if !filepath.IsAbs(path) {
			return fmt.Errorf("bat returned a non-absolute native config path %q", path)
		}
	}
	data, err := nativeRead(path)
	if err != nil {
		return err
	}
	return nativeWrite(path, nativeReplaceLines(data, func(s string) bool { return nativeBatTheme.MatchString(s) }, "--theme="+strconv.Quote(name)))
}
