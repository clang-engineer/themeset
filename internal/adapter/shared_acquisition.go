package adapter

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// Fetch the current official source on every apply. Never execute downloaded
// content; the tool adapter validates its native format before changing config.
func filesAcquireAsset(env model.Environment, source string) ([]byte, error) {
	return filesAcquireAssetWithClient(env, source, filesAssetHTTPClient)
}

func filesAcquireAssetWithClient(_ model.Environment, source string, client *http.Client) ([]byte, error) {
	location, err := url.Parse(source)
	if err != nil || location.Scheme != "https" || location.Host == "" || location.User != nil {
		return nil, errors.New("theme asset source must be an absolute HTTPS URL without credentials")
	}
	response, err := client.Get(source)
	if err != nil {
		return nil, fmt.Errorf("download theme asset %s: %w", source, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download theme asset %s: HTTP %d", source, response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, filesAssetMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read theme asset %s: %w", source, err)
	}
	if len(data) > filesAssetMaxBytes {
		return nil, fmt.Errorf("theme asset exceeds %d bytes: %s", filesAssetMaxBytes, source)
	}
	return data, nil
}
