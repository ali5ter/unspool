package thumbnail

import (
	"strings"
	"testing"
)

func TestCheckDependency(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		if err := CheckDependency(); err != nil {
			t.Skipf("chafa not on this test machine's PATH: %v", err)
		}
	})

	t.Run("absent", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir()) // a directory guaranteed to contain no "chafa"
		err := CheckDependency()
		if err == nil {
			t.Fatal("CheckDependency() = nil with an empty PATH, want an error")
		}
		if !strings.Contains(err.Error(), "chafa") {
			t.Fatalf("CheckDependency() error = %q, want it to mention chafa", err.Error())
		}
	})
}
