package app

import (
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func validFixtureAdapter(id string) model.Adapter {
	return model.Adapter{
		ID: id, Binary: id,
		Resolve: func(model.ThemePort, model.Environment) (model.Port, bool, error) {
			return model.Port{}, false, nil
		},
		Apply: func(model.Port, model.Environment) error { return nil },
	}
}

func TestSelectToolsRejectsInvalidAdapterRegistry(t *testing.T) {
	tests := []struct {
		name     string
		adapters []model.Adapter
		wantErr  string
	}{
		{name: "empty ID", adapters: []model.Adapter{validFixtureAdapter("")}, wantErr: "empty tool ID"},
		{name: "missing binary", adapters: []model.Adapter{{ID: "broken", Resolve: validFixtureAdapter("broken").Resolve, Apply: validFixtureAdapter("broken").Apply}}, wantErr: "adapter \"broken\" is incomplete"},
		{name: "missing resolver", adapters: []model.Adapter{{ID: "broken", Binary: "broken", Apply: validFixtureAdapter("broken").Apply}}, wantErr: "adapter \"broken\" is incomplete"},
		{name: "missing apply", adapters: []model.Adapter{{ID: "broken", Binary: "broken", Resolve: validFixtureAdapter("broken").Resolve}}, wantErr: "adapter \"broken\" is incomplete"},
		{name: "duplicate ID", adapters: []model.Adapter{validFixtureAdapter("same"), validFixtureAdapter("same")}, wantErr: "duplicate tool ID \"same\""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := selectTools(nil, test.adapters)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("selectTools() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestSelectToolsKeepsStableIDOrder(t *testing.T) {
	selected, err := selectTools(nil, []model.Adapter{validFixtureAdapter("zeta"), validFixtureAdapter("alpha")})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].ID != "alpha" || selected[1].ID != "zeta" {
		t.Fatalf("selected adapters = %+v, want stable alphabetical order", selected)
	}
}
