package server

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/parser"
)

// Server представляет VLESS сервер с состоянием
type Server struct {
	// Конфигурация сервера
	*parser.VLESSConfig

	// Состояние
	Active            bool          `json:"active"`
	LastSeen          time.Time     `json:"last_seen"`
	RTT               time.Duration `json:"rtt"`
	ActiveConnections int32         `json:"active_connections"` // атомарный счётчик
}

// NewServer создаёт новый Server из VLESS конфигурации
func NewServer(config *parser.VLESSConfig) *Server {
	return &Server{
		VLESSConfig: config,
		Active:      false,
		LastSeen:    time.Now(),
	}
}

// IncrementConnections увеличивает счётчик активных соединений
func (s *Server) IncrementConnections() {
	atomic.AddInt32(&s.ActiveConnections, 1)
}

// DecrementConnections уменьшает счётчик активных соединений
func (s *Server) DecrementConnections() {
	atomic.AddInt32(&s.ActiveConnections, -1)
}

// GetConnections возвращает текущее количество активных соединений
func (s *Server) GetConnections() int32 {
	return atomic.LoadInt32(&s.ActiveConnections)
}

// SetActive устанавливает активность сервера
func (s *Server) SetActive(active bool) {
	s.Active = active
	s.LastSeen = time.Now()
}

// UpdateRTT обновляет время отклика
func (s *Server) UpdateRTT(rtt time.Duration) {
	s.RTT = rtt
	s.LastSeen = time.Now()
}

// String возвращает строковое представление сервера
func (s *Server) String() string {
	status := "inactive"
	if s.Active {
		status = "active"
	}
	return fmt.Sprintf(
		"[%s] %s (connections: %d, rtt: %v)",
		status,
		s.VLESSConfig.String(),
		s.GetConnections(),
		s.RTT,
	)
}
