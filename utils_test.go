package flow

import "testing"

func TestGenerateHashBasic(t *testing.T) {
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	v := GenerateHash("test")
	if v != expected {
		t.Fatalf("expected %q; received %q", expected, v)
	}
}

func TestGenerateHashMultiple(t *testing.T) {
	expected := "ecd71870d1963316a97e3ac3408c9835ad8cf0f3c1bc703527c30265534f75ae"
	v := GenerateHash("test", "123")
	if v != expected {
		t.Fatalf("expected %q; received %q", expected, v)
	}
}
