package balancer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
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
	balancer    Balancer

	mu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// Graceful shutdown
	shuttingDown atomic.Bool
}

// NewManager создаёт новый менеджер
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:      ctx,
		cancel:   cancel,
		balancer: NewRoundRobinBalancer(),
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

	log.Printf("Manager: added %d servers", len(configs))
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
		m.updateBalancer()
	}

	m.checker.Start(m.ctx)

	go m.periodicBalancerUpdate(interval)
}

// periodicBalancerUpdate периодически обновляет список серверов
func (m *Manager) periodicBalancerUpdate(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.shuttingDown.Load() {
				return
			}
			m.updateBalancer()
		case <-m.ctx.Done():
			return
		}
	}
}

// updateBalancer обновляет список активных серверов
func (m *Manager) updateBalancer() {
	if m.checker == nil {
		return
	}

	activeServers := m.checker.GetActiveServers()
	m.balancer.Update(activeServers)

	if !m.shuttingDown.Load() {
		log.Printf("Balancer: %d active servers", len(activeServers))
	}
}

// StartProxy запускает SOCKS5 прокси
func (m *Manager) StartProxy(addr string) error {
	m.proxyServer = proxy.NewSocks5Server(m.getDialer)
	return m.proxyServer.Start(addr)
}

// Shutdown выполняет graceful shutdown
func (m *Manager) Shutdown() {
	log.Println("Manager: initiating graceful shutdown...")
	m.shuttingDown.Store(true)

	// 1. Останавливаем health checker
	if m.checker != nil {
		log.Println("Manager: stopping health checker...")
		m.checker.Stop()
	}

	// 2. Отменяем контекст
	m.cancel()

	// 3. Останавливаем SOCKS5 прокси (ждёт активные соединения)
	if m.proxyServer != nil {
		log.Println("Manager: stopping SOCKS5 proxy...")
		m.proxyServer.Stop()
	}

	log.Println("Manager: shutdown complete")
}

// Stop останавливает все компоненты (для обратной совместимости)
func (m *Manager) Stop() {
	m.Shutdown()
}

// getDialer возвращает dialer для сервера выбранного балансировщиком
func (m *Manager) getDialer() (vless.Dialer, error) {
	srv := m.balancer.Pick()
	if srv == nil {
		return nil, fmt.Errorf("no active servers available")
	}

	if !m.shuttingDown.Load() {
		log.Printf("Balancer: selected %s (RTT: %v, conns: %d)",
			srv.Name, srv.GetRTT(), srv.GetConnections())
	}

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

	stats["balancer_type"] = "round-robin"
	stats["shutting_down"] = m.shuttingDown.Load()

	if m.proxyServer != nil {
		proxyStats := m.proxyServer.GetStats()
		stats["proxy_active_connections"] = proxyStats["active_connections"]
		stats["proxy_total_connections"] = proxyStats["total_connections"]
	}

	return stats
}

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
