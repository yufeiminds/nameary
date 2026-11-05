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

func TestCleanControlCharacters(t *testing.T) {
	in := "a\nb\tc\rd\x00e\x1b[2Jf\u009bg\u202ah\u2028i\u2029j"
	got := Clean(in)
	if got != "a b c de[2Jfgh i j" {
		t.Fatalf("got %q", got)
	}
}
