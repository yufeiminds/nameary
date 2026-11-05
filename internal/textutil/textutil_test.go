package textutil

import "testing"

func TestClean(t *testing.T) {
	in := "acme\x1b[2J\nname\t\u202eoverride\x00"
	got := Clean(in)
	want := "acme[2J name override"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
