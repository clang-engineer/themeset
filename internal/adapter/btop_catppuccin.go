package adapter

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

const (
	filesBtopCatppuccinBase          = "https://raw.githubusercontent.com/catppuccin/btop/f437574b600f1c6d932627050b15ff5153b58fa3/"
	filesBtopCatppuccinSource        = filesBtopCatppuccinBase + "themes/catppuccin_mocha.theme"
	filesBtopCatppuccinDigest        = "4c7469ab9bda282c1ff606aca37c9c0b84cd0cb66d041e4e7e1fed39791f4644"
	filesBtopCatppuccinLicense       = filesBtopCatppuccinBase + "LICENSE"
	filesBtopCatppuccinLicenseDigest = "814096d2c34cc216c624738a49356f32b7237733b4f7edb0685f4e50ef5074ba"
)

func filesResolveBtopCatppuccin() (model.Port, bool, error) {
	return model.Port{Kind: "official", Name: "catppuccin_mocha", Source: filesBtopCatppuccinSource}, true, nil
}

func filesAcquireBtopCatppuccin(env model.Environment) ([]byte, error) {
	data, err := filesAcquireAsset(env, filesBtopCatppuccinSource, filesBtopCatppuccinDigest)
	if err != nil {
		return nil, fmt.Errorf("acquire btop Catppuccin Mocha: %w", err)
	}
	if err := filesBtopTheme(data); err != nil {
		return nil, fmt.Errorf("invalid btop Catppuccin Mocha: %w", err)
	}
	if _, err := filesAcquireAsset(env, filesBtopCatppuccinLicense, filesBtopCatppuccinLicenseDigest); err != nil {
		return nil, fmt.Errorf("acquire btop Catppuccin Mocha license: %w", err)
	}
	return data, nil
}
