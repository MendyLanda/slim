//go:build darwin

package system

import "testing"

func TestIsPFAlreadyEnabledOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want bool
	}{
		{
			name: "pf already enabled",
			out:  "No ALTQ support in kernel\npfctl: pf already enabled",
			want: true,
		},
		{
			name: "case insensitive",
			out:  "PF Already Enabled",
			want: true,
		},
		{
			name: "different error",
			out:  "pfctl: syntax error",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPFAlreadyEnabledOutput(tt.out)
			if got != tt.want {
				t.Fatalf("isPFAlreadyEnabledOutput(%q) = %v, want %v", tt.out, got, tt.want)
			}
		})
	}
}

func TestIsPFEnabledInfoOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want bool
	}{
		{
			name: "enabled status",
			out:  "Status: Enabled for 0 days 00:12:34           Debug: Urgent",
			want: true,
		},
		{
			name: "enabled status case insensitive",
			out:  "status: enabled",
			want: true,
		},
		{
			name: "disabled status",
			out:  "Status: Disabled",
			want: false,
		},
		{
			name: "unrelated output",
			out:  "No ALTQ support in kernel",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPFEnabledInfoOutput(tt.out)
			if got != tt.want {
				t.Fatalf("isPFEnabledInfoOutput(%q) = %v, want %v", tt.out, got, tt.want)
			}
		})
	}
}

func TestParsePFEnableToken(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "standard output",
			out:  "pf enabled\nToken : 1272727272727272727",
			want: "1272727272727272727",
		},
		{
			name: "extra spaces",
			out:  "Status: Enabled\nToken:   9999",
			want: "9999",
		},
		{
			name: "missing token",
			out:  "pf already enabled",
			want: "",
		},
		{
			name: "malformed token line",
			out:  "Token 12345",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePFEnableToken(tt.out)
			if got != tt.want {
				t.Fatalf("parsePFEnableToken(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

func TestHasPFReferenceToken(t *testing.T) {
	tests := []struct {
		name  string
		out   string
		token string
		want  bool
	}{
		{
			name:  "token present on line",
			out:   "PID 1234 token 55555\nPID 8888 token 99999",
			token: "55555",
			want:  true,
		},
		{
			name:  "token absent",
			out:   "PID 1234 token 55555\nPID 8888 token 99999",
			token: "11111",
			want:  false,
		},
		{
			name:  "empty token",
			out:   "PID 1234 token 55555",
			token: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasPFReferenceToken(tt.out, tt.token)
			if got != tt.want {
				t.Fatalf("hasPFReferenceToken(%q, %q) = %v, want %v", tt.out, tt.token, got, tt.want)
			}
		})
	}
}
