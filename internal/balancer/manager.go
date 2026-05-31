package balancer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/downloader"
	"github.com/FirdavsMF/vpn-balancer/internal/health"
	"github.com/FirdavsMF/vpn-balancer/internal/parser"
	"github.com/FirdavsMF/vpn-balancer/internal/proxy"
	"github.com/FirdavsMF/vpn-balancer/internal/server"
	"github.com/FirdavsMF/vpn-balancer/internal/vless"
)

// Manager управляет всеми компонентами VPN балансировщика
type Manager struct {
	servers   map[string]*server.Server // key = address:port:uuid
	serversMu sync.RWMutex
	sources   []string

	checker     *health.Checker
	proxyServer *proxy.Socks5Server
	balancer    Balancer

	ctx    context.Context
	cancel context.CancelFunc

	shuttingDown atomic.Bool
}

// NewManager создаёт новый менеджер
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:      ctx,
		cancel:   cancel,
		balancer: NewRoundRobinBalancer(),
		servers:  make(map[string]*server.Server),
	}
}

// AddServersFromURLs загружает и добавляет серверы из URL
func (m *Manager) AddServersFromURLs(urls []string) error {
	m.sources = urls
	return m.loadServers(urls)
}

// loadServers загружает серверы из источников
func (m *Manager) loadServers(sources []string) error {
	urls, err := downloader.FetchAll(sources)
	if err != nil {
		return fmt.Errorf("failed to fetch sources: %w", err)
	}

	configs, err := parseURLs(urls)
	if err != nil {
		return fmt.Errorf("failed to parse URLs: %w", err)
	}

	m.serversMu.Lock()
	defer m.serversMu.Unlock()

	newCount := 0
	updatedCount := 0

	// Отслеживаем какие серверы есть в новой загрузке
	seenKeys := make(map[string]bool)

	for _, cfg := range configs {
		key := serverKey(cfg)
		seenKeys[key] = true

		if existing, ok := m.servers[key]; ok {
			// Сервер уже существует, обновляем если нужно
			if existing.Address != cfg.Address || existing.Port != cfg.Port {
				updatedCount++
			}
		} else {
			// Новый сервер
			srv, err := server.NewServer(cfg)
			if err != nil {
				log.Printf("Failed to create server %s: %v", cfg.Name, err)
				continue
			}
			m.servers[key] = srv
			newCount++
		}
	}

	// Помечаем отсутствующие серверы как неактивные
	removedCount := 0
	for key, srv := range m.servers {
		if !seenKeys[key] {
			srv.SetActive(false)
			removedCount++
			log.Printf("Server removed from config: %s", srv.Name)
		}
	}

	log.Printf("Reload: %d new, %d updated, %d removed, %d total",
		newCount, updatedCount, removedCount, len(m.servers))

	// Обновляем health checker если запущен
	if m.checker != nil {
		serverList := m.getServerList()
		m.checker.UpdateServers(serverList)
	}

	return nil
}

// Reload перезагружает конфигурацию из источников
func (m *Manager) Reload() error {
	log.Println("Manager: reloading configuration...")

	if len(m.sources) == 0 {
		return fmt.Errorf("no sources configured")
	}

	return m.loadServers(m.sources)
}

// getServerList возвращает список всех серверов
func (m *Manager) getServerList() []*server.Server {
	m.serversMu.RLock()
	defer m.serversMu.RUnlock()

	servers := make([]*server.Server, 0, len(m.servers))
	for _, srv := range m.servers {
		servers = append(servers, srv)
	}
	return servers
}

// StartHealthChecker запускает проверку здоровья
func (m *Manager) StartHealthChecker(interval, timeout time.Duration) {
	serverList := m.getServerList()
	m.checker = health.NewChecker(interval, timeout)
	m.checker.UpdateServers(serverList)

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

	if m.checker != nil {
		m.checker.Stop()
	}

	m.cancel()

	if m.proxyServer != nil {
		m.proxyServer.Stop()
	}

	log.Println("Manager: shutdown complete")
}

func (m *Manager) Stop() {
	m.Shutdown()
}

func (m *Manager) getDialer() (vless.Dialer, error) {
	srv := m.balancer.Pick()
	if srv == nil {
		return nil, fmt.Errorf("no active servers available")
	}

	return vless.NewVLESSDialer(srv.VLESSConfig)
}

// GetStats возвращает статистику
func (m *Manager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if m.checker != nil {
		stats = m.checker.GetStats()
	}

	m.serversMu.RLock()
	stats["total_servers_loaded"] = len(m.servers)
	m.serversMu.RUnlock()

	stats["balancer_type"] = "round-robin"
	stats["shutting_down"] = m.shuttingDown.Load()

	if m.proxyServer != nil {
		proxyStats := m.proxyServer.GetStats()
		stats["proxy_active_connections"] = proxyStats["active_connections"]
		stats["proxy_total_connections"] = proxyStats["total_connections"]
	}

	return stats
}

// serverKey создаёт уникальный ключ для сервера
func serverKey(cfg *parser.VLESSConfig) string {
	return fmt.Sprintf("%s:%d:%s", cfg.Address, cfg.Port, cfg.UUID[:8])
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
