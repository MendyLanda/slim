package cmd

import "testing"

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "myapp", want: "myapp"},
		{input: "myapp.internal", want: "myapp"},
		{input: "myapp.internal.", want: "myapp"},
		{input: "MYAPP.INTERNAL", want: "myapp"},
		{input: "  myapp.internal  ", want: "myapp"},
		{input: "my-app", want: "my-app"},
	}

	for _, tt := range tests {
		got := normalizeName(tt.input)
		if got != tt.want {
			t.Fatalf("normalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
