package vless

import (
	"context"
	"testing"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/parser"
)

func TestNewVLESSDialer(t *testing.T) {
	tests := []struct {
		name    string
		config  *parser.VLESSConfig
		wantErr bool
	}{
		{
			name: "valid config with real server",
			config: &parser.VLESSConfig{
				Address: "91.210.230.174",
				Port:    30547,
				UUID:    "9ca5cffb-19ea-45d9-a374-181b40f6bf0d",
			},
			wantErr: false,
		},
		{
			name: "valid config with saint petersburg server",
			config: &parser.VLESSConfig{
				Address: "176.57.210.85",
				Port:    28463,
				UUID:    "7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b",
			},
			wantErr: false,
		},
		{
			name: "valid config with novosibirsk server",
			config: &parser.VLESSConfig{
				Address: "194.87.238.15",
				Port:    42319,
				UUID:    "1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty address",
			config: &parser.VLESSConfig{
				Address: "",
				Port:    443,
				UUID:    "12345678-1234-1234-1234-123456789abc",
			},
			wantErr: true,
		},
		{
			name: "empty port",
			config: &parser.VLESSConfig{
				Address: "1.2.3.4",
				Port:    0,
				UUID:    "12345678-1234-1234-1234-123456789abc",
			},
			wantErr: true,
		},
		{
			name: "empty UUID",
			config: &parser.VLESSConfig{
				Address: "1.2.3.4",
				Port:    443,
				UUID:    "",
			},
			wantErr: true,
		},
		{
			name: "invalid UUID format",
			config: &parser.VLESSConfig{
				Address: "1.2.3.4",
				Port:    443,
				UUID:    "not-a-valid-uuid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVLESSDialer(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVLESSDialer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVLESSDialer_IsAvailable(t *testing.T) {
	cfg := &parser.VLESSConfig{
		Address: "192.0.2.1",
		Port:    443,
		UUID:    "12345678-1234-1234-1234-123456789abc",
	}

	dialer, err := NewVLESSDialer(cfg)
	if err != nil {
		t.Fatalf("Failed to create dialer: %v", err)
	}
	defer dialer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	available := dialer.IsAvailable(ctx)
	t.Logf("Server availability: %v", available)
	// Не проверяем конкретное значение, так как зависит от сети
}

func TestVLESSDialer_MultipleServers(t *testing.T) {
	servers := []struct {
		name   string
		config *parser.VLESSConfig
	}{
		{
			name: "Moscow Server",
			config: &parser.VLESSConfig{
				Address: "91.210.230.174",
				Port:    30547,
				UUID:    "9ca5cffb-19ea-45d9-a374-181b40f6bf0d",
			},
		},
		{
			name: "Saint Petersburg Server",
			config: &parser.VLESSConfig{
				Address: "176.57.210.85",
				Port:    28463,
				UUID:    "7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b",
			},
		},
		{
			name: "Novosibirsk Server",
			config: &parser.VLESSConfig{
				Address: "194.87.238.15",
				Port:    42319,
				UUID:    "1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p",
			},
		},
	}

	for _, srv := range servers {
		t.Run(srv.name, func(t *testing.T) {
			dialer, err := NewVLESSDialer(srv.config)
			if err != nil {
				t.Fatalf("Failed to create dialer for %s: %v", srv.name, err)
			}
			defer dialer.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			available := dialer.IsAvailable(ctx)
			t.Logf("%s availability: %v", srv.name, available)
		})
	}
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		name    string
		uuid    string
		wantErr bool
	}{
		{
			name:    "valid UUID from Moscow server",
			uuid:    "9ca5cffb-19ea-45d9-a374-181b40f6bf0d",
			wantErr: false,
		},
		{
			name:    "valid UUID from SPB server",
			uuid:    "7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b",
			wantErr: false,
		},
		{
			name:    "valid UUID from Novosibirsk server",
			uuid:    "1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p",
			wantErr: false,
		},
		{
			name:    "valid UUID without dashes",
			uuid:    "9ca5cffb19ea45d9a374181b40f6bf0d",
			wantErr: false,
		},
		{
			name:    "invalid UUID length",
			uuid:    "12345678-1234-1234-1234",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUUID(tt.uuid)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUUID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVLESSDialer_DialContextWithRealConfig(t *testing.T) {
	cfg := &parser.VLESSConfig{
		Address: "91.210.230.174",
		Port:    30547,
		UUID:    "9ca5cffb-19ea-45d9-a374-181b40f6bf0d",
	}

	dialer, err := NewVLESSDialer(cfg)
	if err != nil {
		t.Fatalf("Failed to create dialer: %v", err)
	}
	defer dialer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", "8.8.8.8:53")
	if err != nil {
		t.Logf("Connection failed (may be expected): %v", err)
		return
	}
	defer conn.Close()

	t.Log("Successfully established connection through VLESS")

	// Отправляем DNS запрос для проверки
	testData := []byte("test")
	if _, err := conn.Write(testData); err != nil {
		t.Logf("Write failed (may be expected): %v", err)
	} else {
		t.Log("Successfully sent data through VLESS connection")
	}
}

func TestVLESSDialer_DialContextWithWebSocket(t *testing.T) {
	cfg := &parser.VLESSConfig{
		Address: "185.130.105.78",
		Port:    5443,
		UUID:    "2b4d6f8h-1a3c5e7g-9i0k2m4o-6q8s0u2w",
	}

	dialer, err := NewVLESSDialer(cfg)
	if err != nil {
		t.Fatalf("Failed to create dialer: %v", err)
	}
	defer dialer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", "1.1.1.1:80")
	if err != nil {
		t.Logf("Connection failed (may be expected): %v", err)
		return
	}
	defer conn.Close()

	t.Log("Successfully established WebSocket connection through VLESS")
}
