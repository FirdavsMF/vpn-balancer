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
	balancer    Balancer

	mu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
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

	log.Printf("Added %d servers", len(configs))
	return nil
}

// StartHealthChecker запускает проверку здоровья
func (m *Manager) StartHealthChecker(interval, timeout time.Duration) {
	m.checker = health.NewChecker(interval, timeout)
	m.checker.UpdateServers(m.servers)

	// При изменении статуса обновляем балансировщик
	m.checker.OnStatusChange = func(srv *server.Server, oldActive bool) {
		if srv.IsActive() {
			log.Printf("✅ Server UP: %s (RTT: %v)", srv.Name, srv.GetRTT())
		} else {
			log.Printf("❌ Server DOWN: %s", srv.Name)
		}
		// Обновляем список активных серверов в балансировщике
		m.updateBalancer()
	}

	m.checker.Start(m.ctx)

	// Запускаем периодическое обновление балансировщика
	go m.periodicBalancerUpdate(interval)
}

// periodicBalancerUpdate периодически обновляет список серверов в балансировщике
func (m *Manager) periodicBalancerUpdate(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateBalancer()
		case <-m.ctx.Done():
			return
		}
	}
}

// updateBalancer обновляет список активных серверов в балансировщике
func (m *Manager) updateBalancer() {
	if m.checker == nil {
		return
	}

	activeServers := m.checker.GetActiveServers()
	m.balancer.Update(activeServers)

	log.Printf("Balancer updated: %d active servers", len(activeServers))
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

// getDialer возвращает dialer для сервера выбранного балансировщиком
func (m *Manager) getDialer() (vless.Dialer, error) {
	// Выбираем сервер через балансировщик
	srv := m.balancer.Pick()
	if srv == nil {
		return nil, fmt.Errorf("no active servers available")
	}

	log.Printf("Balancer: selected server %s (RTT: %v, connections: %d)",
		srv.Name, srv.GetRTT(), srv.GetConnections())

	// Создаём новый dialer для выбранного сервера
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
