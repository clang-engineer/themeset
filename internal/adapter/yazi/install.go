package yazi

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"os"
	"path/filepath"
)

// Only flavors created by themeset may be refreshed. A partial independent
// flavor can be completed only when its existing files match today's source.
func filesInstallOfficialYaziFlavor(root, source string, assets map[string][]byte) error {
	info, err := os.Lstat(root)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if exists && !info.IsDir() {
		return fmt.Errorf("refusing to replace non-directory Yazi flavor %s", root)
	}
	owned := false
	if exists {
		owned, err = filesYaziMatchesSource(root, source)
		if err != nil {
			return err
		}
		marker, err := shared.Read(filepath.Join(root, filesYaziSourceMarker), true)
		if err != nil {
			return err
		}
		if marker != nil && !owned {
			return fmt.Errorf("conflicting Yazi flavor source marker in %s", root)
		}
	}
	for name, data := range assets {
		if filepath.Base(name) != name || name == "." || name == filesYaziSourceMarker || len(data) == 0 {
			return fmt.Errorf("invalid Yazi flavor asset %q", name)
		}
		if exists && !owned {
			previous, err := shared.Read(filepath.Join(root, name), true)
			if err != nil {
				return err
			}
			if previous != nil && !bytes.Equal(previous, data) {
				return fmt.Errorf("refusing to replace independent Yazi flavor %s", filepath.Join(root, name))
			}
		}
	}
	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".themeset-flavor-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if exists {
		if err := os.CopyFS(stage, os.DirFS(root)); err != nil {
			return fmt.Errorf("stage existing Yazi flavor: %w", err)
		}
	}
	for name, data := range assets {
		if err := shared.Atomic(filepath.Join(stage, name), data); err != nil {
			return err
		}
	}
	if err := shared.Atomic(filepath.Join(stage, filesYaziSourceMarker), []byte(source+"\n")); err != nil {
		return err
	}
	if !exists {
		return os.Rename(stage, root)
	}
	backup, err := os.MkdirTemp(parent, ".themeset-flavor-previous-*")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(root, backup); err != nil {
		return err
	}
	if err := os.Rename(stage, root); err != nil {
		if restoreErr := os.Rename(backup, root); restoreErr != nil {
			return fmt.Errorf("install Yazi flavor: %w; restore previous flavor from %s: %v", err, backup, restoreErr)
		}
		return err
	}
	return os.RemoveAll(backup)
}
