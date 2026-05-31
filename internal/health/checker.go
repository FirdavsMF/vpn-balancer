package health

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/server"
)

// Checker выполняет периодическую проверку здоровья серверов
type Checker struct {
	interval time.Duration
	timeout  time.Duration
	servers  []*server.Server
	mu       sync.RWMutex
	wg       sync.WaitGroup
	stopCh   chan struct{}

	totalChecks  int64
	failedChecks int64
	muStats      sync.RWMutex

	OnStatusChange func(srv *server.Server, oldActive bool)
}

// NewChecker создаёт новый health checker
func NewChecker(interval, timeout time.Duration) *Checker {
	return &Checker{
		interval: interval,
		timeout:  timeout,
		stopCh:   make(chan struct{}),
	}
}

// UpdateServers обновляет список серверов для проверки
func (c *Checker) UpdateServers(servers []*server.Server) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.servers = servers
	log.Printf("Health checker: updated server list, now monitoring %d servers", len(servers))
}

// Start запускает периодическую проверку
func (c *Checker) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.run(ctx)
	log.Printf("Health checker started (interval: %v, timeout: %v)", c.interval, c.timeout)
}

// Stop останавливает проверки
func (c *Checker) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	log.Println("Health checker stopped")
}

func (c *Checker) run(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.checkAllServers(ctx)

	for {
		select {
		case <-ticker.C:
			c.checkAllServers(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Checker) checkAllServers(ctx context.Context) {
	c.mu.RLock()
	servers := make([]*server.Server, len(c.servers))
	copy(servers, c.servers)
	c.mu.RUnlock()

	if len(servers) == 0 {
		return
	}

	log.Printf("Health check: starting check for %d servers", len(servers))

	semaphore := make(chan struct{}, 10)
	var wg sync.WaitGroup

	activeCount := 0
	var muActive sync.Mutex

	for _, srv := range servers {
		wg.Add(1)
		go func(s *server.Server) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			oldActive := s.IsActive()
			isActive := c.checkServer(ctx, s)

			s.SetActive(isActive)

			muActive.Lock()
			if isActive {
				activeCount++
			}
			c.muStats.Lock()
			c.totalChecks++
			if !isActive {
				c.failedChecks++
			}
			c.muStats.Unlock()
			muActive.Unlock()

			if c.OnStatusChange != nil && oldActive != isActive {
				c.OnStatusChange(s, oldActive)
			}
		}(srv)
	}

	wg.Wait()

	log.Printf("Health check completed: %d/%d servers active", activeCount, len(servers))
}

func (c *Checker) checkServer(ctx context.Context, srv *server.Server) bool {
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()

	conn, err := srv.Connect(checkCtx, "1.1.1.1:53")
	if err != nil {
		log.Printf("Health check failed for %s: %v", srv.Name, err)
		return false
	}

	rtt := time.Since(start)
	srv.UpdateRTT(rtt)
	conn.Close()

	log.Printf("Health check OK for %s (RTT: %v)", srv.Name, rtt)
	return true
}

// GetStats возвращает статистику проверок
func (c *Checker) GetStats() map[string]interface{} {
	c.mu.RLock()
	totalServers := len(c.servers)
	activeServers := 0
	for _, s := range c.servers {
		if s.IsActive() {
			activeServers++
		}
	}
	c.mu.RUnlock()

	c.muStats.RLock()
	total := c.totalChecks
	failed := c.failedChecks
	c.muStats.RUnlock()

	return map[string]interface{}{
		"total_servers":  totalServers,
		"active_servers": activeServers,
		"total_checks":   total,
		"failed_checks":  failed,
		"interval":       c.interval.String(),
		"timeout":        c.timeout.String(),
	}
}

// GetActiveServers возвращает список активных серверов
func (c *Checker) GetActiveServers() []*server.Server {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var active []*server.Server
	for _, s := range c.servers {
		if s.IsActive() {
			active = append(active, s)
		}
	}
	return active
}

// GetServersByRTT возвращает серверы отсортированные по RTT
func (c *Checker) GetServersByRTT() []*server.Server {
	c.mu.RLock()
	servers := make([]*server.Server, len(c.servers))
	copy(servers, c.servers)
	c.mu.RUnlock()

	// Сортировка пузырьком по RTT
	for i := 0; i < len(servers); i++ {
		for j := i + 1; j < len(servers); j++ {
			if servers[i].GetRTT() > servers[j].GetRTT() {
				servers[i], servers[j] = servers[j], servers[i]
			}
		}
	}

	return servers
}

// String возвращает строковое представление
func (c *Checker) String() string {
	stats := c.GetStats()
	return fmt.Sprintf(
		"Health Checker: %v active / %v total (checks: %v, failed: %v)",
		stats["active_servers"],
		stats["total_servers"],
		stats["total_checks"],
		stats["failed_checks"],
	)
}
