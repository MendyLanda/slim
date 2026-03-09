package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"os"
	"strconv"
	"strings"

	"github.com/kamranahmedse/slim/internal/cert"
	"github.com/kamranahmedse/slim/internal/config"
	"github.com/kamranahmedse/slim/internal/log"
	"golang.org/x/net/http2"
	"golang.org/x/sync/singleflight"
)

var (
	HTTPAddr  = ":0"
	HTTPSAddr = ":0"

	ensureLeafCertFn = cert.EnsureLeafCert
	loadLeafTLSFn    = cert.LoadLeafTLS
)

type Server struct {
	cfg           *config.Config
	cfgMu         sync.RWMutex
	routes        map[string]*domainRouter
	knownDomains  map[string]struct{}
	defaultDomain string
	httpAddr      string
	httpsAddr     string
	httpServer    *http.Server
	tlsServer     *http.Server
	transport     *http.Transport
	certCache     map[string]*tls.Certificate
	certMu        sync.RWMutex
	certGroup     singleflight.Group
}

func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg:          cfg,
		httpAddr:     HTTPAddr,
		httpsAddr:    HTTPSAddr,
		transport:    newUpstreamTransport(),
		routes:       make(map[string]*domainRouter),
		knownDomains: make(map[string]struct{}),
		certCache:    make(map[string]*tls.Certificate),
	}
}

func (s *Server) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	var name string
	if hello.ServerName == "" {
		name = s.defaultConfiguredDomain()
		if name == "" {
			return nil, fmt.Errorf("no domains configured")
		}
	} else {
		var ok bool
		name, ok = localDomainFromHost(hello.ServerName)
		if !ok {
			return nil, fmt.Errorf("unsupported server name %q", hello.ServerName)
		}
	}

	if !s.isKnownDomain(name) {
		return nil, fmt.Errorf("domain %s.test is not configured", name)
	}

	if tlsCert := s.cachedCertificate(name); tlsCert != nil {
		return tlsCert, nil
	}

	val, err, _ := s.certGroup.Do(name, func() (any, error) {
		if tlsCert := s.cachedCertificate(name); tlsCert != nil {
			return tlsCert, nil
		}

		if err := ensureLeafCertFn(name); err != nil {
			return nil, fmt.Errorf("ensuring cert for %s: %w", name, err)
		}

		tlsCert, err := loadLeafTLSFn(name)
		if err != nil {
			return nil, err
		}

		s.certMu.Lock()
		s.certCache[name] = tlsCert
		s.certMu.Unlock()

		return tlsCert, nil
	})
	if err != nil {
		return nil, err
	}

	tlsCert, ok := val.(*tls.Certificate)
	if !ok {
		return nil, fmt.Errorf("invalid certificate cache entry for %s", name)
	}
	return tlsCert, nil
}

func (s *Server) Start() error {
	if err := s.applyConfig(s.cfg); err != nil {
		return err
	}

	handler := buildHandler(s)

	s.httpServer = &http.Server{
		Addr:              s.httpAddr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       2 * time.Hour,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
	}

	s.tlsServer = &http.Server{
		Addr:              s.httpsAddr,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Hour,
		Handler:           handler,
		TLSConfig: &tls.Config{
			GetCertificate: s.getCertificate,
		},
	}

	if err := http2.ConfigureServer(s.tlsServer, nil); err != nil {
		return fmt.Errorf("configuring HTTP/2: %w", err)
	}

	httpLn, err := net.Listen("tcp", s.httpAddr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.httpAddr, err)
	}

	tlsLn, err := net.Listen("tcp", s.httpsAddr)
	if err != nil {
		httpLn.Close()
		return fmt.Errorf("listening on %s: %w", s.httpsAddr, err)
	}

	tlsLn = tls.NewListener(tlsLn, s.tlsServer.TLSConfig)

	httpPort := httpLn.Addr().(*net.TCPAddr).Port
	httpsPort := tlsLn.Addr().(*net.TCPAddr).Port
	if err := WriteProxyPorts(httpPort, httpsPort); err != nil {
		httpLn.Close()
		tlsLn.Close()
		return fmt.Errorf("writing proxy ports: %w", err)
	}

	log.Info("HTTP  listening on %s (redirects to HTTPS)", httpLn.Addr())
	log.Info("HTTPS listening on %s", tlsLn.Addr())

	s.cfgMu.RLock()
	domains := append([]config.Domain(nil), s.cfg.Domains...)
	s.cfgMu.RUnlock()

	for _, d := range domains {
		log.Info("  %s.test → localhost:%d", d.Name, d.Port)
		for _, r := range d.Routes {
			log.Info("    %s → localhost:%d", r.Path, r.Port)
		}
	}

	errCh := make(chan error, 2)
	go func() { errCh <- s.httpServer.Serve(httpLn) }()
	go func() { errCh <- s.tlsServer.Serve(tlsLn) }()

	var retErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && err != http.ErrServerClosed && retErr == nil {
			retErr = err
		}
	}
	return retErr
}

func (s *Server) Shutdown(ctx context.Context) error {
	var firstErr error

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.tlsServer != nil {
		if err := s.tlsServer.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (s *Server) ReloadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if err := s.applyConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (s *Server) applyConfig(cfg *config.Config) error {
	routes := make(map[string]*domainRouter, len(cfg.Domains))
	knownDomains := make(map[string]struct{}, len(cfg.Domains))
	certCache := make(map[string]*tls.Certificate, len(cfg.Domains))
	defaultDomain := ""

	for i, d := range cfg.Domains {
		if i == 0 {
			defaultDomain = d.Name
		}

		if err := ensureLeafCertFn(d.Name); err != nil {
			return fmt.Errorf("ensuring cert for %s: %w", d.Name, err)
		}
		tlsCert, err := loadLeafTLSFn(d.Name)
		if err != nil {
			return fmt.Errorf("loading cert for %s: %w", d.Name, err)
		}

		router := &domainRouter{
			defaultPort:    d.Port,
			defaultHandler: newDomainProxy(d.Port, s.transport, cfg.Cors),
		}

		for _, r := range d.Routes {
			router.pathRoutes = append(router.pathRoutes, pathRoute{
				prefix:  r.Path,
				port:    r.Port,
				handler: http.StripPrefix(r.Path, newDomainProxy(r.Port, s.transport, cfg.Cors)),
			})
		}
		sort.Slice(router.pathRoutes, func(i, j int) bool {
			return len(router.pathRoutes[i].prefix) > len(router.pathRoutes[j].prefix)
		})

		routes[d.Name] = router
		knownDomains[d.Name] = struct{}{}
		certCache[d.Name] = tlsCert
	}

	s.cfgMu.Lock()
	s.cfg = cfg
	s.routes = routes
	s.knownDomains = knownDomains
	s.defaultDomain = defaultDomain
	s.cfgMu.Unlock()

	s.certMu.Lock()
	s.certCache = certCache
	s.certMu.Unlock()

	return nil
}

func (s *Server) cachedCertificate(name string) *tls.Certificate {
	s.certMu.RLock()
	defer s.certMu.RUnlock()
	return s.certCache[name]
}

func (s *Server) isKnownDomain(name string) bool {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	_, ok := s.knownDomains[name]
	return ok
}

func (s *Server) defaultConfiguredDomain() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.defaultDomain
}

// WriteProxyPorts persists the OS-assigned proxy ports so port forwarding can
// read them.
func WriteProxyPorts(httpPort, httpsPort int) error {
	data := fmt.Sprintf("%d %d\n", httpPort, httpsPort)
	return os.WriteFile(config.ProxyPortsPath(), []byte(data), 0644)
}

// ReadProxyPorts returns the HTTP and HTTPS ports the proxy is listening on.
func ReadProxyPorts() (httpPort, httpsPort int, err error) {
	data, err := os.ReadFile(config.ProxyPortsPath())
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(strings.TrimSpace(string(data)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid proxy ports file")
	}
	httpPort, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid HTTP port: %w", err)
	}
	httpsPort, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid HTTPS port: %w", err)
	}
	return httpPort, httpsPort, nil
}

// RemoveProxyPorts cleans up the proxy ports file.
func RemoveProxyPorts() {
	_ = os.Remove(config.ProxyPortsPath())
}

func newUpstreamTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 512
	transport.MaxIdleConnsPerHost = 128
	transport.MaxConnsPerHost = 256
	transport.IdleConnTimeout = 2 * time.Hour
	return transport
}
