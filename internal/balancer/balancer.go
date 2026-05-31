package balancer

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/FirdavsMF/vpn-balancer/internal/server"
)

// Balancer определяет интерфейс для выбора сервера
type Balancer interface {
	// Pick выбирает сервер для нового соединения
	Pick() *server.Server

	// Update обновляет список доступных серверов
	Update(servers []*server.Server)

	// Stats возвращает статистику балансировщика
	Stats() map[string]interface{}
}

// RoundRobinBalancer реализует алгоритм Round Robin
type RoundRobinBalancer struct {
	servers []*server.Server
	counter uint64
	mu      sync.RWMutex
}

// NewRoundRobinBalancer создаёт новый Round Robin балансировщик
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// Pick выбирает следующий сервер по Round Robin
func (rr *RoundRobinBalancer) Pick() *server.Server {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if len(rr.servers) == 0 {
		log.Println("Balancer: no servers available")
		return nil
	}

	// Атомарно инкрементируем счётчик и получаем индекс
	index := atomic.AddUint64(&rr.counter, 1) - 1
	selected := rr.servers[index%uint64(len(rr.servers))]

	log.Printf("RoundRobin: picked server %s (%d/%d)",
		selected.Name, index%uint64(len(rr.servers))+1, len(rr.servers))

	return selected
}

// Update обновляет список серверов
func (rr *RoundRobinBalancer) Update(servers []*server.Server) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	// Фильтруем только активные серверы
	var activeServers []*server.Server
	for _, s := range servers {
		if s.IsActive() {
			activeServers = append(activeServers, s)
		}
	}

	rr.servers = activeServers
	log.Printf("RoundRobin balancer updated: %d active servers", len(activeServers))
}

// Stats возвращает статистику
func (rr *RoundRobinBalancer) Stats() map[string]interface{} {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return map[string]interface{}{
		"type":          "round-robin",
		"total_servers": len(rr.servers),
		"total_picks":   atomic.LoadUint64(&rr.counter),
	}
}

// LeastConnBalancer выбирает сервер с наименьшим количеством соединений
type LeastConnBalancer struct {
	servers []*server.Server
	mu      sync.RWMutex
}

// NewLeastConnBalancer создаёт новый Least Connection балансировщик
func NewLeastConnBalancer() *LeastConnBalancer {
	return &LeastConnBalancer{}
}

// Pick выбирает сервер с наименьшим количеством соединений
func (lc *LeastConnBalancer) Pick() *server.Server {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if len(lc.servers) == 0 {
		log.Println("Balancer: no servers available")
		return nil
	}

	// Находим сервер с минимальным количеством соединений
	var bestServer *server.Server
	minConns := int32(^uint32(0) >> 1) // Max int32

	for _, s := range lc.servers {
		conns := s.GetConnections()
		if conns < minConns {
			minConns = conns
			bestServer = s
		}
	}

	log.Printf("LeastConn: picked server %s (connections: %d)",
		bestServer.Name, bestServer.GetConnections())

	return bestServer
}

// Update обновляет список серверов
func (lc *LeastConnBalancer) Update(servers []*server.Server) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	var activeServers []*server.Server
	for _, s := range servers {
		if s.IsActive() {
			activeServers = append(activeServers, s)
		}
	}

	lc.servers = activeServers
	log.Printf("LeastConn balancer updated: %d active servers", len(activeServers))
}

// Stats возвращает статистику
func (lc *LeastConnBalancer) Stats() map[string]interface{} {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	totalConns := int32(0)
	for _, s := range lc.servers {
		totalConns += s.GetConnections()
	}

	return map[string]interface{}{
		"type":              "least-connections",
		"total_servers":     len(lc.servers),
		"total_connections": totalConns,
	}
}
