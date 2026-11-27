package shared

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
	tomledit "github.com/pelletier/go-toml/v2/unstable/edit"
	"gopkg.in/yaml.v3"
)

func Expand(path string, env model.Environment) string {
	if path == "~" {
		return env.Home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(env.Home, path[2:])
	}
	return path
}

func ConfigHome(env model.Environment) string {
	if env.ConfigHome != "" {
		return Expand(env.ConfigHome, env)
	}
	return filepath.Join(env.Home, ".config")
}

// Read only regular files, following symlinks rather than replacing them. In
// particular, a dangling symlink is an error, not a missing configuration file.
func Read(path string, optional bool) ([]byte, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) && optional {
		if _, linkErr := os.Lstat(path); errors.Is(linkErr, os.ErrNotExist) {
			return nil, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.ReadFile(path)
}

// The rename is on the destination filesystem. Existing modes and symlink
// objects are retained; new files are private until explicitly shared by users.
func Atomic(path string, data []byte) error {
	mode := os.FileMode(0600)
	newFile := false
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			info, err = os.Stat(path)
			if err != nil {
				return err
			}
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		mode = info.Mode().Perm()
		if err := preserveBackup(path, mode); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	} else {
		newFile = true
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".themeset-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if newFile {
		createdMarker, err := preserveCreatedMarker(path)
		if err != nil {
			return err
		}
		if err := os.Rename(f.Name(), path); err != nil {
			if createdMarker {
				os.Remove(path + ".themeset.bak.new")
			}
			return err
		}
		return nil
	}
	return os.Rename(f.Name(), path)
}

func preserveCreatedMarker(path string) (bool, error) {
	marker := path + ".themeset.bak.new"
	f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, f.Close()
}

func preserveBackup(path string, mode os.FileMode) error {
	backup := path + ".themeset.bak"
	if info, err := os.Lstat(backup); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace unsafe backup %s", backup)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := writeNewFile(backup, original, mode); err != nil {
		return fmt.Errorf("backup %s: %w", path, err)
	}
	return nil
}

func writeNewFile(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}

// Rollback restores all regular-file backups beneath root. Symlinks are not
// traversed; unsafe backup entries are reported and left untouched.
func Rollback(root string) (int, error) {
	count := 0
	var failures []error
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrPermission) || errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			failures = append(failures, walkErr)
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".themeset.bak") && !strings.HasSuffix(entry.Name(), ".themeset.bak.new") {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			failures = append(failures, fmt.Errorf("unsafe backup %s", path))
			return nil
		}
		created := strings.HasSuffix(path, ".themeset.bak.new")
		suffix := ".themeset.bak"
		if created {
			suffix = ".themeset.bak.new"
		}
		target := strings.TrimSuffix(path, suffix)
		if info, err := os.Lstat(target); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
			failures = append(failures, fmt.Errorf("refusing to restore over unsafe target %s", target))
			return nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
			return nil
		}
		if created {
			if _, targetErr := os.Lstat(target); targetErr == nil {
				err = os.Remove(target)
			} else if errors.Is(targetErr, os.ErrNotExist) {
				err = nil
			} else {
				err = targetErr
			}
		} else {
			var data []byte
			data, err = os.ReadFile(path)
			if err == nil {
				err = AtomicRestore(target, data, info.Mode().Perm())
			}
		}
		if err == nil {
			err = os.Remove(path)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("restore %s: %w", target, err))
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		failures = append(failures, err)
	}
	return count, errors.Join(failures...)
}

func AtomicRestore(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".themeset-rollback-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func TOML(data []byte) (map[string]any, error) {
	result := make(map[string]any)
	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func YAML(data []byte) (map[string]any, error) {
	result := make(map[string]any)
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&result); err != nil && err != io.EOF {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("configuration must contain one YAML document")
		}
		return nil, err
	}
	if result == nil {
		result = make(map[string]any)
	}
	return result, nil
}

func Table(config map[string]any, key string) (map[string]any, error) {
	if value, exists := config[key]; exists && value != nil {
		if table, ok := value.(map[string]any); ok {
			return table, nil
		}
		return nil, fmt.Errorf("%s must be a table/mapping", key)
	}
	table := make(map[string]any)
	config[key] = table
	return table, nil
}

func Merge(dst, src map[string]any) {
	for key, value := range src {
		incoming, incomingMap := value.(map[string]any)
		existing, existingMap := dst[key].(map[string]any)
		if incomingMap && existingMap {
			Merge(existing, incoming)
		} else {
			dst[key] = value
		}
	}
}

func EditTOML(path string, change func(map[string]any) error) error {
	data, err := Read(path, true)
	if err != nil {
		return err
	}
	config, err := TOML(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	original := cloneTOMLMap(config)
	if err := change(config); err != nil {
		return err
	}
	document, err := tomledit.Parse(data)
	if err != nil {
		return err
	}
	if err := editTOMLMap(document, original, config, nil); err != nil {
		return err
	}
	return Atomic(path, []byte(document.String()))
}

func cloneTOMLMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		if nested, ok := value.(map[string]any); ok {
			clone[key] = cloneTOMLMap(nested)
		} else {
			clone[key] = value
		}
	}
	return clone
}

func editTOMLMap(document *tomledit.Document, before, after map[string]any, parent []string) error {
	keys := make([]string, 0, len(before)+len(after))
	seen := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range after {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		path := append(append([]string(nil), parent...), key)
		oldValue, existed := before[key]
		newValue, remains := after[key]
		if !remains {
			if !document.Delete(path) {
				return fmt.Errorf("TOML key %q disappeared during edit", strings.Join(path, "."))
			}
			continue
		}
		oldTable, wasTable := oldValue.(map[string]any)
		newTable, isTable := newValue.(map[string]any)
		if existed && wasTable && isTable {
			if err := editTOMLMap(document, oldTable, newTable, path); err != nil {
				return err
			}
			continue
		}
		if existed && reflect.DeepEqual(oldValue, newValue) {
			continue
		}
		if err := document.Set(path, newValue); err != nil {
			return err
		}
	}
	return nil
}

func Custom(port model.Port, env model.Environment) ([]byte, error) {
	if port.URL != "" {
		if port.File != "" || port.Kind != "custom" {
			return nil, errors.New("remote custom port requires URL only and custom kind")
		}
		return AcquireAsset(env, port.URL)
	}
	if port.File == "" {
		return nil, errors.New("custom port requires an app-native file or HTTPS URL")
	}
	return Read(Expand(port.File, env), false)
}

func Name(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}
