package proxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/vless"
)

// Socks5Server реализует SOCKS5 прокси сервер
type Socks5Server struct {
	listener  net.Listener
	getDialer func() (vless.Dialer, error)
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	timeout   time.Duration
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
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	log.Printf("SOCKS5 proxy server started on %s", addr)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop останавливает SOCKS5 сервер
func (s *Socks5Server) Stop() error {
	s.cancel()
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	log.Println("SOCKS5 proxy server stopped")
	return nil
}

// acceptLoop принимает входящие соединения
func (s *Socks5Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// handleConnection обрабатывает одно SOCKS5 соединение
func (s *Socks5Server) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// Устанавливаем deadline
	clientConn.SetDeadline(time.Now().Add(s.timeout))

	// Шаг 1: SOCKS5 Handshake
	if err := s.handleHandshake(clientConn); err != nil {
		log.Printf("SOCKS5 handshake failed: %v", err)
		return
	}

	// Шаг 2: SOCKS5 Request
	targetAddr, err := s.handleRequest(clientConn)
	if err != nil {
		log.Printf("SOCKS5 request failed: %v", err)
		return
	}

	// Сбрасываем deadline
	clientConn.SetDeadline(time.Time{})

	// Шаг 3: Подключаемся к целевому хосту через VLESS
	dialer, err := s.getDialer()
	if err != nil {
		log.Printf("Failed to get VLESS dialer: %v", err)
		s.sendReply(clientConn, 0x01, nil) // General failure
		return
	}

	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()

	remoteConn, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to %s via VLESS: %v", targetAddr, err)
		s.sendReply(clientConn, 0x04, nil) // Host unreachable
		return
	}
	defer remoteConn.Close()

	// Отправляем успешный ответ
	s.sendReply(clientConn, 0x00, remoteConn.LocalAddr())

	log.Printf("SOCKS5: %s -> %s (via VLESS)", clientConn.RemoteAddr(), targetAddr)

	// Шаг 4: Проксируем данные
	s.relay(clientConn, remoteConn)
}

// handleHandshake выполняет SOCKS5 рукопожатие
func (s *Socks5Server) handleHandshake(conn net.Conn) error {
	// Читаем версию и количество методов аутентификации
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("failed to read handshake: %w", err)
	}

	version := buf[0]
	if version != 0x05 {
		return fmt.Errorf("unsupported SOCKS version: %d", version)
	}

	nmethods := buf[1]

	// Читаем методы аутентификации
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("failed to read auth methods: %w", err)
	}

	// Поддерживаем только "no authentication" (0x00)
	hasNoAuth := false
	for _, m := range methods {
		if m == 0x00 {
			hasNoAuth = true
			break
		}
	}

	if !hasNoAuth {
		conn.Write([]byte{0x05, 0xFF}) // No acceptable methods
		return fmt.Errorf("no acceptable authentication method")
	}

	// Отправляем ответ: версия 5, метод 0 (no auth)
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return fmt.Errorf("failed to write auth response: %w", err)
	}

	return nil
}

// handleRequest обрабатывает SOCKS5 запрос
func (s *Socks5Server) handleRequest(conn net.Conn) (string, error) {
	// Читаем первые 4 байта: версия, команда, зарезервировано, тип адреса
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", fmt.Errorf("failed to read request: %w", err)
	}

	version := buf[0]
	command := buf[1]
	addrType := buf[3]

	if version != 0x05 {
		return "", fmt.Errorf("invalid SOCKS version: %d", version)
	}

	// Поддерживаем только CONNECT
	if command != 0x01 {
		s.sendReply(conn, 0x07, nil) // Command not supported
		return "", fmt.Errorf("unsupported command: %d", command)
	}

	// Читаем адрес в зависимости от типа
	var host string
	var port uint16

	switch addrType {
	case 0x01: // IPv4
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("failed to read IPv4 address: %w", err)
		}
		host = net.IP(addr).String()

	case 0x03: // Доменное имя
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", fmt.Errorf("failed to read domain length: %w", err)
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", fmt.Errorf("failed to read domain: %w", err)
		}
		host = string(domain)

	case 0x04: // IPv6
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("failed to read IPv6 address: %w", err)
		}
		host = net.IP(addr).String()

	default:
		s.sendReply(conn, 0x08, nil) // Address type not supported
		return "", fmt.Errorf("unsupported address type: %d", addrType)
	}

	// Читаем порт (2 байта)
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", fmt.Errorf("failed to read port: %w", err)
	}
	port = binary.BigEndian.Uint16(portBuf)

	targetAddr := fmt.Sprintf("%s:%d", host, port)
	return targetAddr, nil
}

// sendReply отправляет SOCKS5 ответ
func (s *Socks5Server) sendReply(conn net.Conn, rep byte, bindAddr net.Addr) {
	reply := []byte{0x05, rep, 0x00, 0x01} // Версия, ответ, зарезервировано, тип адреса (IPv4)

	if bindAddr != nil {
		// Извлекаем IP и порт из адреса
		host, portStr, err := net.SplitHostPort(bindAddr.String())
		if err == nil {
			ip := net.ParseIP(host)
			if ip != nil && ip.To4() != nil {
				reply = append(reply, ip.To4()...)
				var p int
				fmt.Sscanf(portStr, "%d", &p)
				reply = append(reply, byte(p>>8), byte(p&0xFF))
			}
		}
	} else {
		// Bind address: 0.0.0.0:0
		reply = append(reply, 0, 0, 0, 0, 0, 0)
	}

	conn.Write(reply)
}

// relay копирует данные между клиентом и удалённым сервером
func (s *Socks5Server) relay(client, remote net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Клиент -> Удалённый сервер
	go func() {
		defer wg.Done()
		io.Copy(remote, client)
		// Сигнализируем что данных больше не будет
		if tcpConn, ok := remote.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Удалённый сервер -> Клиент
	go func() {
		defer wg.Done()
		io.Copy(client, remote)
	}()

	wg.Wait()
}
