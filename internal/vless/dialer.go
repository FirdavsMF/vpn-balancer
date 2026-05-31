package vless

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/parser"
)

// Dialer определяет интерфейс для установки соединений через VLESS
type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	Close() error
	IsAvailable(ctx context.Context) bool
}

// VLESSDialer реализует Dialer интерфейс
type VLESSDialer struct {
	config  *parser.VLESSConfig
	address string
}

// NewVLESSDialer создаёт новый VLESS dialer
func NewVLESSDialer(cfg *parser.VLESSConfig) (Dialer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("VLESS config is nil")
	}

	if cfg.Address == "" {
		return nil, fmt.Errorf("server address is empty")
	}

	if cfg.Port == 0 {
		return nil, fmt.Errorf("server port is empty")
	}

	if cfg.UUID == "" {
		return nil, fmt.Errorf("UUID is empty")
	}

	// Простая проверка UUID (длина после удаления дефисов)
	cleanUUID := strings.ReplaceAll(cfg.UUID, "-", "")
	if len(cleanUUID) != 32 {
		return nil, fmt.Errorf("invalid UUID format: %s", cfg.UUID)
	}

	address := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)

	return &VLESSDialer{
		config:  cfg,
		address: address,
	}, nil
}

// DialContext устанавливает TCP соединение с target через VLESS прокси
func (d *VLESSDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("unsupported network type: %s (only tcp is supported)", network)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Устанавливаем TCP соединение с VLESS сервером
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", d.address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to VLESS server %s: %w", d.address, err)
	}

	// Выполняем VLESS handshake
	if err := d.handshake(ctx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("VLESS handshake failed: %w", err)
	}

	// Отправляем команду на подключение к целевому хосту
	if err := d.connect(ctx, conn, addr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("VLESS connect failed: %w", err)
	}

	return conn, nil
}

// handshake выполняет VLESS handshake с сервером
func (d *VLESSDialer) handshake(ctx context.Context, conn net.Conn) error {
	// Отправляем версию протокола
	version := []byte{0x00}
	if _, err := conn.Write(version); err != nil {
		return fmt.Errorf("failed to write protocol version: %w", err)
	}

	// Преобразуем UUID из строки в 16 байт
	uuidBytes, err := parseUUID(d.config.UUID)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	if _, err := conn.Write(uuidBytes); err != nil {
		return fmt.Errorf("failed to write UUID: %w", err)
	}

	return nil
}

// connect отправляет команду на подключение к целевому адресу
func (d *VLESSDialer) connect(ctx context.Context, conn net.Conn, targetAddr string) error {
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return fmt.Errorf("invalid target address %s: %w", targetAddr, err)
	}

	var addrType byte
	var addrBytes []byte

	ip := net.ParseIP(host)
	if ip != nil {
		if ip.To4() != nil {
			addrType = 0x01
			addrBytes = ip.To4()
		} else {
			addrType = 0x04
			addrBytes = ip.To16()
		}
	} else {
		addrType = 0x02
		addrBytes = []byte(host)
	}

	var cmd []byte
	cmd = append(cmd, addrType)

	if addrType == 0x02 {
		cmd = append(cmd, byte(len(addrBytes)))
	}

	cmd = append(cmd, addrBytes...)

	portNum := uint16(0)
	if p, err := parsePort(port); err == nil {
		portNum = p
	}
	cmd = append(cmd, byte(portNum>>8), byte(portNum&0xFF))

	if _, err := conn.Write(cmd); err != nil {
		return fmt.Errorf("failed to write connect command: %w", err)
	}

	return nil
}

// Close закрывает dialer
func (d *VLESSDialer) Close() error {
	return nil
}

// IsAvailable проверяет доступность сервера
func (d *VLESSDialer) IsAvailable(ctx context.Context) bool {
	conn, err := d.DialContext(ctx, "tcp", "8.8.8.8:53")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// parseUUID преобразует строку UUID в 16 байт
func parseUUID(uuid string) ([]byte, error) {
	// Удаляем дефисы
	clean := strings.ReplaceAll(uuid, "-", "")

	if len(clean) != 32 {
		return nil, fmt.Errorf("invalid UUID length: expected 32 hex chars, got %d", len(clean))
	}

	result := make([]byte, 16)
	for i := 0; i < 16; i++ {
		hexByte := clean[i*2 : i*2+2]
		var val byte
		if _, err := fmt.Sscanf(hexByte, "%02x", &val); err != nil {
			// Если не hex, просто копируем байты как есть
			result[i] = clean[i*2]
			if i*2+1 < len(clean) {
				result[i] = result[i]<<4 | clean[i*2+1]
			}
		} else {
			result[i] = val
		}
	}

	return result, nil
}

// parsePort парсит порт из строки
func parsePort(port string) (uint16, error) {
	var p int
	if _, err := fmt.Sscanf(port, "%d", &p); err != nil {
		return 0, err
	}
	if p < 0 || p > 65535 {
		return 0, fmt.Errorf("port out of range: %d", p)
	}
	return uint16(p), nil
}
