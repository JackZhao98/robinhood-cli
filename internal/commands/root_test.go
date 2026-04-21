package commands

import "testing"

func TestRootDefaultFormatIsTable(t *testing.T) {
	cmd := NewRoot()
	f := cmd.PersistentFlags().Lookup("format")
	if f == nil {
		t.Fatal("missing format flag")
	}
	if got, want := f.DefValue, "table"; got != want {
		t.Fatalf("format default = %q, want %q", got, want)
	}
}
