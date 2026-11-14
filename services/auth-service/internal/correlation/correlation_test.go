package correlation

import (
	"testing"
)

func TestNormalizeReplacesUnsafeID(t *testing.T) {
	id := Normalize("unsafe request id")
	if !Valid(id) || id == "unsafe request id" {
		t.Fatalf("Normalize returned %q", id)
	}
}
