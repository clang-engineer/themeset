package adapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/clang-engineer/themeset/internal/model"
)

const filesAssetMaxBytes = 4 << 20

var filesAssetHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return errors.New("theme asset redirect must use HTTPS")
		}
		if len(via) >= 10 {
			return errors.New("too many theme asset redirects")
		}
		return nil
	},
}

// Only apply calls acquisition. Source and digest come from the registered,
// pinned recipe, never from an executable command or an untrusted web page.
func filesAcquireAsset(env model.Environment, source, digest string) ([]byte, error) {
	return filesAcquireAssetWithClient(env, source, digest, filesAssetHTTPClient)
}

func filesAcquireAssetWithClient(env model.Environment, source, digest string, client *http.Client) ([]byte, error) {
	expected, err := hex.DecodeString(digest)
	if err != nil || len(expected) != sha256.Size {
		return nil, errors.New("theme asset requires a SHA-256 digest")
	}
	location, err := url.Parse(source)
	if err != nil || location.Scheme != "https" || location.Host == "" || location.User != nil {
		return nil, errors.New("theme asset source must be an absolute HTTPS URL without credentials")
	}
	cache := filepath.Join(env.Home, ".cache", "themeset", "assets", hex.EncodeToString(expected))
	data, err := os.ReadFile(cache)
	if err == nil {
		actual := sha256.Sum256(data)
		if !bytes.Equal(actual[:], expected) {
			return nil, fmt.Errorf("cached theme asset checksum mismatch: %s", cache)
		}
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read theme asset cache: %w", err)
	}
	response, err := client.Get(source)
	if err != nil {
		return nil, fmt.Errorf("download theme asset %s: %w", source, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download theme asset %s: HTTP %d", source, response.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, filesAssetMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read theme asset %s: %w", source, err)
	}
	if len(data) > filesAssetMaxBytes {
		return nil, fmt.Errorf("theme asset exceeds %d bytes: %s", filesAssetMaxBytes, source)
	}
	actual := sha256.Sum256(data)
	if !bytes.Equal(actual[:], expected) {
		return nil, fmt.Errorf("downloaded theme asset checksum mismatch: %s", source)
	}
	if err := filesAtomic(cache, data); err != nil {
		return nil, fmt.Errorf("cache theme asset: %w", err)
	}
	return data, nil
}
