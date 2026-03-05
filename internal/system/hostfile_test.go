package system

import "testing"

func TestLineHasHost(t *testing.T) {
	tests := []struct {
		line     string
		hostname string
		want     bool
	}{
		{"127.0.0.1 myapp.internal # slim", "myapp.internal", true},
		{"127.0.0.1 other.internal # slim", "myapp.internal", false},
		{"127.0.0.1 myapp.internal.extra # slim", "myapp.internal", false},
		{"# comment", "myapp.internal", false},
		{"", "myapp.internal", false},
		{"127.0.0.1\tmyapp.internal\t# slim", "myapp.internal", true},
	}

	for _, tt := range tests {
		got := lineHasHost(tt.line, tt.hostname)
		if got != tt.want {
			t.Errorf("lineHasHost(%q, %q) = %v, want %v", tt.line, tt.hostname, got, tt.want)
		}
	}
}

func TestHasMarkedEntry(t *testing.T) {
	content := "127.0.0.1 localhost\n127.0.0.1 myapp.internal # slim\n"

	if !HasMarkedEntry(content, "myapp.internal") {
		t.Error("expected to find marked entry for myapp.internal")
	}
	if HasMarkedEntry(content, "other.internal") {
		t.Error("did not expect to find marked entry for other.internal")
	}
	if HasMarkedEntry("", "myapp.internal") {
		t.Error("did not expect to find entry in empty content")
	}
}
