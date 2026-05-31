package vless

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ProxyConnection обёртка над net.Conn для проксирования через VLESS
type ProxyConnection struct {
	serverConn net.Conn
	targetAddr string
	closed     bool
	mu         sync.Mutex
}

// Read читает данные из прокси-соединения
func (pc *ProxyConnection) Read(b []byte) (n int, err error) {
	return pc.serverConn.Read(b)
}

// Write записывает данные в прокси-соединение
func (pc *ProxyConnection) Write(b []byte) (n int, err error) {
	return pc.serverConn.Write(b)
}

// Close закрывает прокси-соединение
func (pc *ProxyConnection) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return nil
	}
	pc.closed = true

	return pc.serverConn.Close()
}

// LocalAddr возвращает локальный адрес
func (pc *ProxyConnection) LocalAddr() net.Addr {
	return pc.serverConn.LocalAddr()
}

// RemoteAddr возвращает удалённый адрес
func (pc *ProxyConnection) RemoteAddr() net.Addr {
	return pc.serverConn.RemoteAddr()
}

// SetDeadline устанавливает deadline
func (pc *ProxyConnection) SetDeadline(t time.Time) error {
	return pc.serverConn.SetDeadline(t)
}

// SetReadDeadline устанавливает deadline для чтения
func (pc *ProxyConnection) SetReadDeadline(t time.Time) error {
	return pc.serverConn.SetReadDeadline(t)
}

// SetWriteDeadline устанавливает deadline для записи
func (pc *ProxyConnection) SetWriteDeadline(t time.Time) error {
	return pc.serverConn.SetWriteDeadline(t)
}

// Relay копирует данные между двумя соединениями
func Relay(ctx context.Context, client, target net.Conn) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(2)

	// Клиент -> Цель
	go func() {
		defer wg.Done()
		_, err := io.Copy(target, client)
		if err != nil {
			errCh <- fmt.Errorf("client->target copy error: %w", err)
		}
	}()

	// Цель -> Клиент
	go func() {
		defer wg.Done()
		_, err := io.Copy(client, target)
		if err != nil {
			errCh <- fmt.Errorf("target->client copy error: %w", err)
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && !isClosedError(err) {
			return err
		}
	}

	return nil
}

// isClosedError проверяет что ошибка связана с закрытием соединения
func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Err.Error() == "use of closed network connection"
	}
	return false
}
