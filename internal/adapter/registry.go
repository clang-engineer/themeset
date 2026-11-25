package adapter

import "github.com/clang-engineer/themeset/internal/model"

func All() []model.Adapter {
	out := nativeAdapters()
	return append(out, fileAdapters()...)
}
