package model

import "os/exec"

// Port references an existing app theme, an app-native file, or an explicitly
// registered remote file. Source is provenance text; URL is the fetch location.
type Port struct {
	Kind              string `json:"kind,omitempty"`
	Name              string `json:"name,omitempty"`
	File              string `json:"file,omitempty"`
	URL               string `json:"url,omitempty"`
	SyntaxURL         string `json:"syntax_url,omitempty"`
	Source            string `json:"source,omitempty"`
	Format            string `json:"format,omitempty"`
	Variant           string `json:"variant,omitempty"`
	LicenseURL        string `json:"license_url,omitempty"`
	AssetURL          string `json:"asset_url,omitempty"`
	LicenseSyntaxURL  string `json:"license_syntax_url,omitempty"`
	StyleKeys         string `json:"style_keys,omitempty"`
	RequiredStyleKeys string `json:"required_style_keys,omitempty"`
}

// ThemePort describes the bundled default for one theme and tool. Native
// candidates are checked in order before the optional verified upstream port.
// A user custom port replaces this entire declaration for that tool.
type ThemePort struct {
	Native       []string `json:"native,omitempty"`
	NativeSource string   `json:"native_source,omitempty"`
	Official     *Port    `json:"official,omitempty"`
}

type Theme struct {
	Custom map[string]Port `json:"custom,omitempty"`
}

type Config struct {
	Tools  []string          `json:"tools,omitempty"`
	Paths  map[string]string `json:"paths,omitempty"`
	Themes map[string]Theme  `json:"themes,omitempty"`
}

type Environment struct {
	Home               string
	ConfigHome         string
	ConfigHomeExplicit bool
	OS                 string
	Paths              map[string]string
}

// Adapter resolves a bundled declaration against installed native providers,
// then its optional verified upstream port. Errors describe inspection failures.
type Adapter struct {
	ID      string
	Binary  string
	Remote  bool // accepts explicit HTTPS theme files, not executable scripts
	Resolve func(ThemePort, Environment) (Port, bool, error)
	Apply   func(Port, Environment) error
}

func (a Adapter) Installed() bool {
	_, err := exec.LookPath(a.Binary)
	return err == nil
}
