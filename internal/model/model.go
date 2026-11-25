package model

import "os/exec"

// Port references an existing app theme or a registered settings file; it never defines a palette.
type Port struct {
	Kind   string `json:"kind,omitempty"`
	Name   string `json:"name,omitempty"`
	File   string `json:"file,omitempty"`
	Source string `json:"source,omitempty"`
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

// Adapter resolves verified native/official ports before any caller-provided fallback.
// Resolve returns false for unsupported/unavailable native themes. Errors describe inspection failures.
type Adapter struct {
	ID      string
	Binary  string
	Resolve func(string, Environment) (Port, bool, error)
	Apply   func(Port, Environment) error
}

func (a Adapter) Installed() bool {
	_, err := exec.LookPath(a.Binary)
	return err == nil
}
