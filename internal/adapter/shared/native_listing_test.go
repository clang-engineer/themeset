package shared

import "testing"

func TestNativeListingsResolveExactNamesAcrossToolFormats(t *testing.T) {
	for _, listing := range []string{
		"Dracula (resources)\nDracula+ (resources)\n",
		"dark\tDracula\nlight\tGitHub\n",
	} {
		if !ListContains([]byte(listing), "Dracula") {
			t.Fatalf("available Dracula was rejected: %q", listing)
		}
		if ListContains([]byte(listing), "Drac") {
			t.Fatalf("prefix matched a different theme: %q", listing)
		}
	}
	if ListContains([]byte("Dracula+ (resources)\n"), "Dracula") {
		t.Fatal("Dracula+ was substituted for Dracula")
	}
}
