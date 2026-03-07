package cmd

import (
	"fmt"
	"net"
	"time"

	"github.com/kamranahmedse/slim/internal/system"
)

var cmdDialTimeoutFn = net.DialTimeout

func ingressPortsReachable() bool {
	for _, port := range []int{80, 443} {
		conn, err := cmdDialTimeoutFn("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
	}
	return true
}

func shouldReloadPortForwarding(pf system.PortForwarder, daemonRunning bool) bool {
	if !pf.IsEnabled() {
		return false
	}
	if !pf.IsLoaded() {
		return true
	}
	return daemonRunning && !ingressPortsReachable()
}
