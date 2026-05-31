package proxy

import (
"context"
"encoding/binary"
"fmt"
"io"
"log"
"net"
"sync"
"sync/atomic"
"time"

"github.com/FirdavsMF/vpn-balancer/internal/vless"
)

// Socks5Server реализует SOCKS5 прокси сервер
type Socks5Server struct {
listener   net.Listener
getDialer  func() (vless.Dialer, error)
ctx        context.Context
cancel     context.CancelFunc
wg         sync.WaitGroup
timeout    time.Duration

activeConnections int64
totalConnections  uint64

shuttingDown atomic.Bool
shutdownWg   sync.WaitGroup
}

// NewSocks5Server создаёт новый SOCKS5 сервер
func NewSocks5Server(getDialer func() (vless.Dialer, error)) *Socks5Server {
ctx, cancel := context.WithCancel(context.Background())
return &Socks5Server{
getDialer: getDialer,
ctx:       ctx,
cancel:    cancel,
timeout:   30 * time.Second,
}
}

// Start запускает SOCKS5 сервер на указанном адресе
func (s *Socks5Server) Start(addr string) error {
// Настраиваем listener с SO_REUSEADDR
lc := net.ListenConfig{
Control: func(network, address string, c syscall.RawConn) error {
var err error
c.Control(func(fd uintptr) {
err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
})
return err
},
}

var err error
s.listener, err = lc.Listen(context.Background(), "tcp", addr)
if err != nil {
return fmt.Errorf("failed to listen on %s: %w", addr, err)
}

log.Printf("SOCKS5 proxy server started on %s", addr)

s.wg.Add(1)
go s.acceptLoop()

return nil
}

// ... остальной код без изменений ...
