package adapter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type ezaDraculaTransport func(*http.Request) (*http.Response, error)

func (f ezaDraculaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func ezaDraculaClient(t *testing.T, transport ezaDraculaTransport) {
	t.Helper()
	previous := filesAssetHTTPClient
	filesAssetHTTPClient = &http.Client{Transport: transport}
	t.Cleanup(func() { filesAssetHTTPClient = previous })
}

func TestEzaDraculaAcquiresCurrentNativeThemeAndLicense(t *testing.T) {
	env := filesTestEnv(t)
	path := filepath.Join(env.Home, "explicit", "theme.yml")
	env.Paths["eza"] = path
	filesTestWrite(t, path, "filenames:\n  Makefile:\n    icon:\n      glyph: KEEP\nfilekinds:\n  directory:\n    is_italic: true\nextensions:\n  mine:\n    filename: {foreground: red}\n")
	requests := 0
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		var body string
		switch r.URL.String() {
		case filesEzaDraculaSource:
			body = ezaDraculaTestTheme
		case filesEzaDraculaLicenseSource:
			body = ezaDraculaTestLicense
		default:
			t.Fatalf("unexpected asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	port, ok, err := filesResolveEza("dracula", env)
	if err != nil || !ok || port.Kind != "official" || requests != 0 {
		t.Fatalf("resolver: %#v, %v; requests=%d", port, err, requests)
	}
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	first := filesTestRead(t, path)
	config, err := filesYAML(first)
	if err != nil {
		t.Fatal(err)
	}
	directory := config["filekinds"].(map[string]any)["directory"].(map[string]any)
	if directory["foreground"] != "#8BE9FD" || directory["is_italic"] != true || config["filenames"].(map[string]any)["Makefile"].(map[string]any)["icon"].(map[string]any)["glyph"] != "KEEP" || config["extensions"].(map[string]any)["mine"] == nil {
		t.Fatalf("native rule merge failed: %#v", config)
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want theme and license", requests)
	}
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 4 || !bytes.Equal(first, filesTestRead(t, path)) {
		t.Fatal("reapply did not fetch latest theme and license or changed effective theme")
	}
}

func TestEzaDraculaFailureDoesNotMutateActiveTheme(t *testing.T) {
	for _, failure := range []string{"download", "invalid schema", "license"} {
		t.Run(failure, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath("eza", env)
			const original = "filenames:\n  precious: {filename: {foreground: red}}\n"
			filesTestWrite(t, path, original)
			ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
				if failure == "download" || (failure == "license" && r.URL.String() == filesEzaDraculaLicenseSource) {
					return nil, fmt.Errorf("offline")
				}
				body := ezaDraculaTestTheme
				if failure == "invalid schema" {
					body = "gui: {theme: {}}\n"
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			port, _, _ := filesResolveEza("dracula", env)
			if err := filesApplyEza(port, env); err == nil {
				t.Fatal("expected acquisition failure")
			}
			if string(filesTestRead(t, path)) != original {
				t.Fatal("failure mutated active theme")
			}
		})
	}
}

func TestEzaDraculaRejectsNonNativeThemeData(t *testing.T) {
	for _, bad := range []string{"", "gui:\n  theme: {}\n", "export EZA_COLORS=blue\n", "colourful: true\nfilekinds: broken\n", strings.Replace(ezaDraculaTestTheme, "directory: {foreground: \"#8BE9FD\"}", "directory: {foreground: false}", 1)} {
		if err := filesValidateEzaDracula([]byte(bad)); err == nil {
			t.Fatalf("accepted non-native YAML: %q", bad)
		}
	}
}

const ezaDraculaTestTheme = `colourful: true
# yellow #F1FA8C
# red    #FF5555
# purple #BD93F9
# pink   #FF79C6
# orange #FFB86C
# green  #50FA7B
# cyan   #8BE9FD
# others:
# dark-blue    #6272A4
# foreground   #F8F8F2
# current_line #44475A

filekinds:
  normal: {foreground: "#F8F8F2"}
  directory: {foreground: "#8BE9FD"}
  symlink: {foreground: "#BD93F9"}
  pipe: {foreground: "#6272A4"}
  block_device: {foreground: "#FF5555"}
  char_device: {foreground: "#FF5555"}
  socket: {foreground: "#44475A"}
  special: {foreground: "#FF79C6"}
  executable: {foreground: "#50FA7B"}
  mount_point: {foreground: "#FFB86C"}

perms:
  user_read: {foreground: "#F8F8F2"}
  user_write: {foreground: "#FFB86C"}
  user_execute_file: {foreground: "#50FA7B"}
  user_execute_other: {foreground: "#50FA7B"}
  group_read: {foreground: "#F8F8F2"}
  group_write: {foreground: "#FFB86C"}
  group_execute: {foreground: "#50FA7B"}
  other_read: {foreground: "#F8F8F2"}
  other_write: {foreground: "#FFB86C"}
  other_execute: {foreground: "#50FA7B"}
  special_user_file: {foreground: "#FF79C6"}
  special_other: {foreground: "#6272A4"}
  attribute: {foreground: "#F8F8F2"}

size:
  major: {foreground: "F1FA8C"}
  minor: {foreground: "#BD93F9"}
  number_byte: {foreground: "#F8F8F2"}
  number_kilo: {foreground: "#F8F8F2"}
  number_mega: {foreground: "#8BE9FD"}
  number_giga: {foreground: "#FF79C6"}
  number_huge: {foreground: "#FF79C6"}
  unit_byte: {foreground: "#F8F8F2"}
  unit_kilo: {foreground: "#8BE9FD"}
  unit_mega: {foreground: "#FF79C6"}
  unit_giga: {foreground: "#FF79C6"}
  unit_huge: {foreground: "#FFB86C"}

users:
  user_you: {foreground: "#F8F8F2"}
  user_root: {foreground: "#FF5555"}
  user_other: {foreground: "#FF79C6"}
  group_yours: {foreground: "#F8F8F2"}
  group_other: {foreground: "#6272A4"}
  group_root: {foreground: "#FF5555"}

links:
  normal: {foreground: "#BD93F9"}
  multi_link_file: {foreground: "#FFB86C"}

git:
  new: {foreground: "#50FA7B"}
  modified: {foreground: "#FFB86C"}
  deleted: {foreground: "#FF5555"}
  renamed: {foreground: "#8BE9FD"}
  typechange: {foreground: "#FF79C6"}
  ignored: {foreground: "#6272A4"}
  conflicted: {foreground: "#FF5555"}

git_repo:
  branch_main: {foreground: "#F8F8F2"}
  branch_other: {foreground: "#FF79C6"}
  git_clean: {foreground: "#50FA7B"}
  git_dirty: {foreground: "#FF5555"}

security_context:
  colon: {foreground: "#6272A4"}
  user: {foreground: "#F8F8F2"}
  role: {foreground: "#FF79C6"}
  typ: {foreground: "#6272A4"}
  range: {foreground: "#FF79C6"}

file_type:
  image: {foreground: "#FFB86C"}
  video: {foreground: "#FF5555"}
  music: {foreground: "#50FA7B"}
  lossless: {foreground: "#50FA7B"}
  crypto: {foreground: "#6272A4"}
  document: {foreground: "#F8F8F2"}
  compressed: {foreground: "#FF79C6"}
  temp: {foreground: "#FF5555"}
  compiled: {foreground: "#8BE9FD"}
  build: {foreground: "#6272A4"}
  source: {foreground: "#8BE9FD"}

punctuation: {foreground: "#6272A4"}
date: {foreground: "#FFB86C"}
inode: {foreground: "#F8F8F2"}
blocks: {foreground: "#F8F8F2"}
header: {foreground: "#F8F8F2"}
octal: {foreground: "#50FA7B"}
flags: {foreground: "#FF79C6"}

symlink_path: {foreground: "#BD93F9"}
control_char: {foreground: "#8BE9FD"}
broken_symlink: {foreground: "#FF5555"}
broken_path_overlay: {foreground: "#6272A4"}
`

const ezaDraculaTestLicense = `The MIT License (MIT)

Copyright (c) 2024 Sandro-Alessio Gierens

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`
