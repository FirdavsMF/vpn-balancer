package server

import (
"context"
"fmt"
"net"
"sync"
"sync/atomic"
"time"

"github.com/FirdavsMF/vpn-balancer/internal/parser"
"github.com/FirdavsMF/vpn-balancer/internal/vless"
)

// Server представляет VLESS сервер с потокобезопасным состоянием
type Server struct {
*parser.VLESSConfig

mu     sync.RWMutex
Active bool
LastSeen time.Time
RTT    time.Duration

ActiveConnections int32 // atomic

dialer vless.Dialer
}

// NewServer создаёт новый Server
func NewServer(config *parser.VLESSConfig) (*Server, error) {
dialer, err := vless.NewVLESSDialer(config)
if err != nil {
return nil, fmt.Errorf("failed to create VLESS dialer: %w", err)
}

return &Server{
VLESSConfig: config,
Active:      false,
LastSeen:    time.Now(),
dialer:      dialer,
}, nil
}

// Connect устанавливает соединение с целевым адресом через VLESS сервер
func (s *Server) Connect(ctx context.Context, targetAddr string) (net.Conn, error) {
if s.dialer == nil {
return nil, fmt.Errorf("VLESS dialer is not initialized")
}

start := time.Now()
conn, err := s.dialer.DialContext(ctx, "tcp", targetAddr)
if err != nil {
return nil, fmt.Errorf("failed to connect to %s via %s: %w",
targetAddr, s.VLESSConfig.Address, err)
}

s.UpdateRTT(time.Since(start))
s.IncrementConnections()

return &trackedConn{
Conn:   conn,
server: s,
}, nil
}

// trackedConn отслеживает закрытие соединения
type trackedConn struct {
net.Conn
server *Server
}

func (tc *trackedConn) Close() error {
err := tc.Conn.Close()
tc.server.DecrementConnections()
return err
}

// IncrementConnections увеличивает счётчик соединений (atomic)
func (s *Server) IncrementConnections() {
atomic.AddInt32(&s.ActiveConnections, 1)
}

// DecrementConnections уменьшает счётчик соединений (atomic)
func (s *Server) DecrementConnections() {
atomic.AddInt32(&s.ActiveConnections, -1)
}

// GetConnections возвращает количество активных соединений (atomic)
func (s *Server) GetConnections() int32 {
return atomic.LoadInt32(&s.ActiveConnections)
}

// SetActive устанавливает статус активности (потокобезопасно)
func (s *Server) SetActive(active bool) {
s.mu.Lock()
defer s.mu.Unlock()
s.Active = active
s.LastSeen = time.Now()
}

// IsActive возвращает статус активности (потокобезопасно)
func (s *Server) IsActive() bool {
s.mu.RLock()
defer s.mu.RUnlock()
return s.Active
}

// UpdateRTT обновляет время отклика (потокобезопасно)
func (s *Server) UpdateRTT(rtt time.Duration) {
s.mu.Lock()
defer s.mu.Unlock()
s.RTT = rtt
s.LastSeen = time.Now()
}

// GetRTT возвращает время отклика (потокобезопасно)
func (s *Server) GetRTT() time.Duration {
s.mu.RLock()
defer s.mu.RUnlock()
return s.RTT
}

// Close закрывает сервер и освобождает ресурсы
func (s *Server) Close() error {
if s.dialer != nil {
return s.dialer.Close()
}
return nil
}

// String возвращает строковое представление
func (s *Server) String() string {
status := "inactive"
if s.IsActive() {
status = "active"
}
return fmt.Sprintf(
"[%s] %s (connections: %d, rtt: %v)",
status,
s.VLESSConfig.String(),
s.GetConnections(),
s.GetRTT(),
)
}
