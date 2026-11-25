package app

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/clang-engineer/themeset/internal/adapter"
	"github.com/clang-engineer/themeset/internal/model"
)

const previewCode = `package main

import "fmt"

// A native theme, not an approximated palette.
func main() {
    names := []string{"terminal", "editor", "shell"}
    for index, name := range names {
        fmt.Printf("%d: %s\n", index + 1, name)
    }
}
`

// renderVisualPreview only inspects installed themes. In particular, registered
// theme files are data, never commands, and no application's config is loaded as
// an executable preview script.
func renderVisualPreview(theme string, config model.Config, env model.Environment, stdout io.Writer) error {
	var inspectionErrors []error
	var ghostty model.Adapter
	for _, candidate := range adapter.All() {
		if candidate.ID == "ghostty" {
			ghostty = candidate
		}
		if candidate.ID != "bat" || !candidate.Installed() {
			continue
		}
		if _, overridden := config.Themes[theme].Custom["bat"]; overridden {
			// Do not show a native syntax palette that apply will override.
			continue
		}
		port, found, err := candidate.Resolve(theme, env)
		if err != nil {
			inspectionErrors = append(inspectionErrors, fmt.Errorf("inspect bat native preview: %q", err.Error()))
			continue
		}
		if !found {
			continue
		}
		out, err := previewBat(port, env)
		if err != nil {
			inspectionErrors = append(inspectionErrors, err)
			continue
		}
		if _, err := fmt.Fprintf(stdout, "Source app: bat; native syntax theme: %q\nProvenance: %q\n\n", port.Name, port.Source); err != nil {
			return errors.Join(append(inspectionErrors, err)...)
		}
		_, err = stdout.Write(out)
		return errors.Join(append(inspectionErrors, err)...)
	}

	_, customGhostty := config.Themes[theme].Custom["ghostty"]
	if !customGhostty && ghostty.Resolve != nil && ghostty.Installed() {
		port, found, err := ghostty.Resolve(theme, env)
		if err != nil {
			inspectionErrors = append(inspectionErrors, fmt.Errorf("inspect Ghostty native preview: %q", err.Error()))
		} else if found {
			path, err := previewGhosttyAsset(port.Name, env)
			if err != nil {
				inspectionErrors = append(inspectionErrors, err)
			} else if path != "" {
				if err := previewGhosttyFile(path, port.Name, "native", port.Source, stdout); err == nil {
					return errors.Join(inspectionErrors...)
				} else {
					inspectionErrors = append(inspectionErrors, err)
				}
			}
		}
	}
	if port, found := config.Themes[theme].Custom["ghostty"]; found {
		path := ""
		var err error
		if port.File != "" {
			path = previewExpand(port.File, env)
		} else if port.Name != "" {
			path, err = previewGhosttyAsset(port.Name, env)
		}
		if err != nil {
			inspectionErrors = append(inspectionErrors, err)
		} else if path != "" {
			if err := previewGhosttyFile(path, port.Name, "registered custom", port.Source, stdout); err == nil {
				return errors.Join(inspectionErrors...)
			} else if errors.Is(err, os.ErrNotExist) {
				// A registered but absent asset is unavailable, not an inspection failure.
			} else {
				inspectionErrors = append(inspectionErrors, err)
			}
		}
	}
	_, err := fmt.Fprintln(stdout, "Preview unavailable: no readable native bat theme or complete Ghostty theme asset for this selection.")
	return errors.Join(append(inspectionErrors, err)...)
}

func previewExpand(path string, env model.Environment) string {
	if path == "~" {
		return env.Home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(env.Home, path[2:])
	}
	if path != "" && !filepath.IsAbs(path) {
		return filepath.Join(env.Home, path)
	}
	return path
}

func previewConfigHome(env model.Environment) string {
	if env.ConfigHome != "" {
		return previewExpand(env.ConfigHome, env)
	}
	return filepath.Join(env.Home, ".config")
}

func previewBat(port model.Port, env model.Environment) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bat", "--no-config", "--theme="+port.Name, "--color=always", "--paging=never", "--style=plain", "--language=go", "--wrap=never", "--terminal-width=80")
	cmd.Dir = env.Home
	// Explicit flags and an isolated HOME/config prevent the user's bat options,
	// pager, or theme override from changing the demo or running another process.
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(key, "BAT_") {
			continue
		}
		switch key {
		case "HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR", "TERM", "COLORTERM", "PAGER":
			continue
		}
		cmd.Env = append(cmd.Env, item)
	}
	cmd.Env = append(cmd.Env, "HOME="+env.Home, "XDG_CONFIG_HOME="+previewConfigHome(env), "XDG_DATA_HOME="+filepath.Join(env.Home, ".local", "share"), "XDG_CACHE_HOME="+filepath.Join(env.Home, ".cache"), "TERM=xterm-256color", "COLORTERM=truecolor")
	cmd.Stdin = strings.NewReader(previewCode)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("render bat native theme %q: %w (diagnostic: %q)", port.Name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Search user assets in Ghostty's lookup order, then installed resources. The
// configured config path supplies its adjacent theme directory as well.
func previewGhosttyDirs(env model.Environment, binary, resources string) []string {
	var dirs []string
	if path := env.Paths["ghostty"]; path != "" {
		dirs = append(dirs, filepath.Join(filepath.Dir(previewExpand(path, env)), "themes"))
	}
	dirs = append(dirs, filepath.Join(previewConfigHome(env), "ghostty", "themes"))
	if env.OS == "darwin" {
		dirs = append(dirs, filepath.Join(env.Home, "Library", "Application Support", "com.mitchellh.ghostty", "themes"))
	}
	if resources != "" {
		dirs = append(dirs, filepath.Join(previewExpand(resources, env), "themes"))
	}
	if binary != "" {
		// Covers both prefix/bin/ghostty and Ghostty.app/Contents/MacOS/ghostty.
		dirs = append(dirs, filepath.Join(filepath.Dir(binary), "..", "share", "ghostty", "themes"))
		if env.OS == "darwin" {
			dirs = append(dirs, filepath.Join(filepath.Dir(binary), "..", "Resources", "ghostty", "themes"))
		}
	}
	if env.OS == "darwin" {
		dirs = append(dirs, filepath.Join(env.Home, "Applications", "Ghostty.app", "Contents", "Resources", "ghostty", "themes"), "/Applications/Ghostty.app/Contents/Resources/ghostty/themes", "/opt/homebrew/share/ghostty/themes")
	}
	dirs = append(dirs, "/usr/local/share/ghostty/themes", "/usr/share/ghostty/themes")
	seen := make(map[string]bool, len(dirs))
	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if !seen[dir] {
			seen[dir] = true
			result = append(result, dir)
		}
	}
	return result
}

func previewGhosttyAsset(name string, env model.Environment) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00\r\n\x1b") {
		return "", fmt.Errorf("invalid Ghostty preview theme name %q", name)
	}
	binary, err := exec.LookPath("ghostty")
	if err != nil && !errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("locate Ghostty executable: %q", err.Error())
	}
	if binary != "" {
		binary, err = filepath.EvalSymlinks(binary)
		if err != nil {
			return "", fmt.Errorf("inspect Ghostty executable: %q", err.Error())
		}
	}
	for _, dir := range previewGhosttyDirs(env, binary, os.Getenv("GHOSTTY_RESOURCES_DIR")) {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", previewAssetError("inspect Ghostty theme asset", path, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("Ghostty theme asset %q is not a regular file", path)
		}
		return path, nil
	}
	return "", nil
}

type previewRGB [3]uint8

type previewPalette struct {
	background previewRGB
	foreground previewRGB
	ansi       [16]previewRGB
}

func previewColor(value string) (previewRGB, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return previewRGB{}, errors.New("preview requires an explicit six-digit RGB color")
	}
	n, err := strconv.ParseUint(value, 16, 24)
	if err != nil {
		return previewRGB{}, errors.New("preview requires an explicit six-digit RGB color")
	}
	return previewRGB{uint8(n >> 16), uint8(n >> 8), uint8(n)}, nil
}

func parsePreviewGhostty(reader io.Reader) (previewPalette, error) {
	var palette previewPalette
	var background, foreground bool
	var present [16]bool
	scanner := bufio.NewScanner(io.LimitReader(reader, 1024*1024+1))
	readBytes := 0
	for scanner.Scan() {
		readBytes += len(scanner.Bytes()) + 1
		if readBytes > 1024*1024 {
			return palette, errors.New("Ghostty theme exceeds preview size limit")
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return palette, errors.New("Ghostty theme contains a non-assignment line")
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "background", "foreground":
			color, err := previewColor(value)
			if err != nil {
				return palette, fmt.Errorf("Ghostty %s: %w", key, err)
			}
			if key == "background" {
				palette.background, background = color, true
			} else {
				palette.foreground, foreground = color, true
			}
		case "palette":
			index, color, found := strings.Cut(value, "=")
			n, err := strconv.Atoi(strings.TrimSpace(index))
			if !found || err != nil || n < 0 || n > 255 {
				return palette, errors.New("Ghostty palette index must be between 0 and 255")
			}
			rgb, err := previewColor(color)
			if err != nil {
				return palette, fmt.Errorf("Ghostty palette slot %d: %w", n, err)
			}
			if n < len(palette.ansi) {
				palette.ansi[n], present[n] = rgb, true
			}
		case "config-file", "theme":
			return palette, errors.New("Ghostty theme includes another asset; preview will not guess inherited colors")
		}
	}
	if err := scanner.Err(); err != nil {
		return palette, fmt.Errorf("read Ghostty theme: %q", err.Error())
	}
	if !background || !foreground {
		return palette, errors.New("Ghostty preview requires explicit background and foreground colors")
	}
	for index, found := range present {
		if !found {
			return palette, fmt.Errorf("Ghostty preview requires explicit ANSI palette slot %d", index)
		}
	}
	return palette, nil
}

// PathError repeats its raw filename in Error(); unwrap it so quoted labels
// cannot be followed by terminal-control bytes from a malicious asset path.
func previewAssetError(action, path string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return fmt.Errorf("%s %q: %w", action, path, err)
}

func previewGhosttyFile(path, name, kind, source string, stdout io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return previewAssetError("inspect Ghostty preview asset", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Ghostty preview asset %q is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return previewAssetError("open Ghostty preview asset", path, err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return previewAssetError("inspect Ghostty preview asset", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Ghostty preview asset %q is not a regular file", path)
	}
	palette, err := parsePreviewGhostty(file)
	if err != nil {
		return fmt.Errorf("parse Ghostty preview asset %q: %w", path, err)
	}
	if _, err := fmt.Fprintf(stdout, "Source app: Ghostty; %s variant: %q\nAsset: %q\n", kind, name, path); err != nil {
		return err
	}
	if source != "" {
		_, err = fmt.Fprintf(stdout, "Provenance: %q\n", source)
	} else {
		_, err = fmt.Fprintln(stdout, "Provenance: installed/local asset; no upstream source registered")
	}
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(stdout, "Ghostty ANSI sample (actual RGB values; not a syntax-highlighting theme)"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "Background: #%02x%02x%02x  Foreground: #%02x%02x%02x\n", palette.background[0], palette.background[1], palette.background[2], palette.foreground[0], palette.foreground[1], palette.foreground[2]); err != nil {
		return err
	}
	bg, fg := palette.background, palette.foreground
	if _, err := fmt.Fprintf(stdout, "\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm", bg[0], bg[1], bg[2], fg[0], fg[1], fg[2]); err != nil {
		return err
	}
	for index, color := range palette.ansi {
		if _, err := fmt.Fprintf(stdout, "\x1b[48;2;%d;%d;%dm %2d \x1b[48;2;%d;%d;%dm", color[0], color[1], color[2], index, bg[0], bg[1], bg[2]); err != nil {
			return err
		}
		if index == 7 || index == 15 {
			if _, err := fmt.Fprintln(stdout); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(stdout, "\x1b[0mANSI token demo: each labeled row uses only its numbered palette slot."); err != nil {
		return err
	}
	for _, token := range []struct {
		index int
		text  string
	}{{4, "func main() {"}, {2, "    message := \"hello, terminal\""}, {3, "    count := 42"}, {8, "    // terminal ANSI colors, not syntax rules"}, {6, "    fmt.Println(message, count)"}, {5, "}"}} {
		color := palette.ansi[token.index]
		if _, err := fmt.Fprintf(stdout, "\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm%2d | %s\x1b[0m\n", bg[0], bg[1], bg[2], color[0], color[1], color[2], token.index, token.text); err != nil {
			return err
		}
	}
	return nil
}
