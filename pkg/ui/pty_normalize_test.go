package ui

import "testing"

func TestNormalizePTYOutput_CRLF(t *testing.T) {
	input := []byte("line 1\r\nline 2\r\n")
	got := normalizePTYOutput(input)
	want := "line 1\nline 2\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestNormalizePTYOutput_StripsBareCR(t *testing.T) {
	input := []byte("hello\rworld")
	got := normalizePTYOutput(input)
	want := "helloworld"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
