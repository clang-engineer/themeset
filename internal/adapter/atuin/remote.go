package atuin

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

// A remote source may refresh only the exact theme it previously installed.
// The local digest tracks ownership, not an upstream version or pinned release.
func filesRemoteAtuinTarget(port model.Port, env model.Environment) (string, string, error) {
	path := filepath.Join(filesAtuinThemes(env), port.Name+".toml")
	marker := filepath.Join(filesAtuinThemes(env), ".themeset-"+port.Name+".source")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, markerErr := os.Lstat(marker); !errors.Is(markerErr, os.ErrNotExist) {
			return "", "", fmt.Errorf("Atuin theme %s has a conflicting ownership marker: %v", path, markerErr)
		}
		return path, marker, nil
	}
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("refusing to replace non-regular Atuin theme %s", path)
	}
	markerInfo, err := os.Lstat(marker)
	if err != nil || !markerInfo.Mode().IsRegular() {
		return "", "", fmt.Errorf("refusing to replace unowned Atuin theme %s", path)
	}
	previous, err := shared.Read(path, false)
	if err != nil {
		return "", "", err
	}
	ownership, err := shared.Read(marker, false)
	if err != nil {
		return "", "", err
	}
	if string(ownership) != filesRemoteAtuinSignature(port.URL, previous) {
		return "", "", fmt.Errorf("refusing to replace modified or differently sourced Atuin theme %s", path)
	}
	return path, marker, nil
}

func filesRemoteAtuinSignature(source string, theme []byte) string {
	return fmt.Sprintf("%x\n%x\n", sha256.Sum256([]byte(source)), sha256.Sum256(theme))
}

func filesWriteRemoteAtuin(port model.Port, env model.Environment, asset []byte) error {
	path, marker, err := filesRemoteAtuinTarget(port, env)
	if err != nil {
		return err
	}
	if err := shared.Atomic(path, asset); err != nil {
		return err
	}
	return shared.Atomic(marker, []byte(filesRemoteAtuinSignature(port.URL, asset)))
}
