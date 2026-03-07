package cmd

import (
	"errors"
	"net"
	"testing"
	"time"
)

type mockCmdPortFwd struct {
	enabled bool
	loaded  bool
}

func (m *mockCmdPortFwd) Enable() error       { return nil }
func (m *mockCmdPortFwd) Disable() error      { return nil }
func (m *mockCmdPortFwd) IsEnabled() bool     { return m.enabled }
func (m *mockCmdPortFwd) IsLoaded() bool      { return m.loaded }
func (m *mockCmdPortFwd) EnsureLoaded() error { return nil }

func TestIngressPortsReachable(t *testing.T) {
	prev := cmdDialTimeoutFn
	defer func() { cmdDialTimeoutFn = prev }()

	cmdDialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
		client, server := net.Pipe()
		_ = server.Close()
		return client, nil
	}
	if !ingressPortsReachable() {
		t.Fatal("expected ingress ports to be reachable")
	}

	cmdDialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, errors.New("refused")
	}
	if ingressPortsReachable() {
		t.Fatal("expected ingress ports to be unreachable")
	}
}

func TestShouldReloadPortForwarding(t *testing.T) {
	prev := cmdDialTimeoutFn
	defer func() { cmdDialTimeoutFn = prev }()

	cmdDialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
		client, server := net.Pipe()
		_ = server.Close()
		return client, nil
	}

	if shouldReloadPortForwarding(&mockCmdPortFwd{enabled: false, loaded: false}, true) {
		t.Fatal("expected no reload when forwarding is not configured")
	}
	if !shouldReloadPortForwarding(&mockCmdPortFwd{enabled: true, loaded: false}, false) {
		t.Fatal("expected reload when configured but not loaded")
	}
	if shouldReloadPortForwarding(&mockCmdPortFwd{enabled: true, loaded: true}, true) {
		t.Fatal("expected no reload when configured, loaded, and ingress is healthy")
	}

	cmdDialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, errors.New("refused")
	}
	if !shouldReloadPortForwarding(&mockCmdPortFwd{enabled: true, loaded: true}, true) {
		t.Fatal("expected reload when ingress is down while daemon is running")
	}
	if shouldReloadPortForwarding(&mockCmdPortFwd{enabled: true, loaded: true}, false) {
		t.Fatal("expected no reload when daemon is not running")
	}
}
