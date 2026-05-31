package balancer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/health"
	"github.com/FirdavsMF/vpn-balancer/internal/parser"
	"github.com/FirdavsMF/vpn-balancer/internal/proxy"
	"github.com/FirdavsMF/vpn-balancer/internal/server"
	"github.com/FirdavsMF/vpn-balancer/internal/vless"
)

// Manager управляет всеми компонентами VPN балансировщика
type Manager struct {
	servers     []*server.Server
	checker     *health.Checker
	proxyServer *proxy.Socks5Server

	mu           sync.RWMutex
	currentIndex int

	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager создаёт новый менеджер
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddServersFromURLs загружает и добавляет серверы из URL
func (m *Manager) AddServersFromURLs(urls []string) error {
	configs, err := parseURLs(urls)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cfg := range configs {
		srv, err := server.NewServer(cfg)
		if err != nil {
			log.Printf("Failed to create server from %s: %v", cfg.Name, err)
			continue
		}
		m.servers = append(m.servers, srv)
	}

	log.Printf("Added %d servers", len(configs))
	return nil
}

// StartHealthChecker запускает проверку здоровья
func (m *Manager) StartHealthChecker(interval, timeout time.Duration) {
	m.checker = health.NewChecker(interval, timeout)
	m.checker.UpdateServers(m.servers)

	m.checker.OnStatusChange = func(srv *server.Server, oldActive bool) {
		if srv.IsActive() {
			log.Printf("✅ Server UP: %s (RTT: %v)", srv.Name, srv.GetRTT())
		} else {
			log.Printf("❌ Server DOWN: %s", srv.Name)
		}
	}

	m.checker.Start(m.ctx)
}

// StartProxy запускает SOCKS5 прокси
func (m *Manager) StartProxy(addr string) error {
	m.proxyServer = proxy.NewSocks5Server(m.getDialer)
	return m.proxyServer.Start(addr)
}

// Stop останавливает все компоненты
func (m *Manager) Stop() {
	if m.proxyServer != nil {
		m.proxyServer.Stop()
	}
	if m.checker != nil {
		m.checker.Stop()
	}
	m.cancel()
}

// getDialer возвращает dialer для активного сервера
func (m *Manager) getDialer() (vless.Dialer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.checker == nil {
		return nil, fmt.Errorf("health checker not started")
	}

	activeServers := m.checker.GetActiveServers()
	if len(activeServers) == 0 {
		return nil, fmt.Errorf("no active servers available")
	}

	srv := activeServers[0]
	return vless.NewVLESSDialer(srv.VLESSConfig)
}

// GetStats возвращает статистику
func (m *Manager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if m.checker != nil {
		stats = m.checker.GetStats()
	}

	m.mu.RLock()
	stats["total_servers_loaded"] = len(m.servers)
	m.mu.RUnlock()

	return stats
}

// parseURLs парсит список VLESS URL
func parseURLs(urls []string) ([]*parser.VLESSConfig, error) {
	var configs []*parser.VLESSConfig

	for _, url := range urls {
		if !strings.HasPrefix(url, "vless://") {
			continue
		}

		cfg, err := parser.Parse(url)
		if err != nil {
			continue
		}

		configs = append(configs, cfg)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no valid VLESS URLs found")
	}

	return configs, nil
}
