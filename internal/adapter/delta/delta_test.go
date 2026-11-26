package delta

import "testing"

func TestRosePineDeltaRejectsUnrelatedFeature(t *testing.T) {
	if _, err := nativeRosePineDeltaMain([]byte("[delta \"other\"]\n\tdark = true\n")); err == nil {
		t.Fatal("accepted unrelated Delta feature")
	}
}
