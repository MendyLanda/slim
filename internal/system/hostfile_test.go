package system

import "testing"

func TestLineHasHost(t *testing.T) {
	tests := []struct {
		line     string
		hostname string
		want     bool
	}{
		{"127.0.0.1 myapp.test # slim", "myapp.test", true},
		{"127.0.0.1 other.test # slim", "myapp.test", false},
		{"127.0.0.1 myapp.test.extra # slim", "myapp.test", false},
		{"# comment", "myapp.test", false},
		{"", "myapp.test", false},
		{"127.0.0.1\tmyapp.test\t# slim", "myapp.test", true},
	}

	for _, tt := range tests {
		got := lineHasHost(tt.line, tt.hostname)
		if got != tt.want {
			t.Errorf("lineHasHost(%q, %q) = %v, want %v", tt.line, tt.hostname, got, tt.want)
		}
	}
}

func TestHasMarkedEntry(t *testing.T) {
	content := "127.0.0.1 localhost\n127.0.0.1 myapp.test # slim\n"

	if !HasMarkedEntry(content, "myapp.test") {
		t.Error("expected to find marked entry for myapp.test")
	}
	if HasMarkedEntry(content, "other.test") {
		t.Error("did not expect to find marked entry for other.test")
	}
	if HasMarkedEntry("", "myapp.test") {
		t.Error("did not expect to find entry in empty content")
	}
}
