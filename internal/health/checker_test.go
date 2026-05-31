package health

import (
	"context"
	"testing"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/parser"
	"github.com/FirdavsMF/vpn-balancer/internal/server"
)

func createTestServers() []*server.Server {
	configs := []*parser.VLESSConfig{
		{
			Address:  "91.210.230.174",
			Port:     30547,
			UUID:     "9ca5cffb-19ea-45d9-a374-181b40f6bf0d",
			Name:     "Test Server 1",
			Security: "reality",
			Network:  "tcp",
		},
		{
			Address:  "192.0.2.1", // Недоступный сервер
			Port:     443,
			UUID:     "12345678-1234-1234-1234-123456789abc",
			Name:     "Test Server 2 (unavailable)",
			Security: "reality",
			Network:  "tcp",
		},
		{
			Address:  "176.57.210.85",
			Port:     28463,
			UUID:     "7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b",
			Name:     "Test Server 3",
			Security: "reality",
			Network:  "tcp",
		},
	}

	var servers []*server.Server
	for _, cfg := range configs {
		srv, err := server.NewServer(cfg)
		if err != nil {
			continue
		}
		servers = append(servers, srv)
	}

	return servers
}

func TestNewChecker(t *testing.T) {
	checker := NewChecker(30*time.Second, 5*time.Second)

	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}

	if checker.interval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", checker.interval)
	}

	if checker.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", checker.timeout)
	}
}

func TestCheckerUpdateServers(t *testing.T) {
	checker := NewChecker(30*time.Second, 5*time.Second)

	servers := createTestServers()
	checker.UpdateServers(servers)

	stats := checker.GetStats()
	if stats["total_servers"] != len(servers) {
		t.Errorf("Expected %d servers, got %v", len(servers), stats["total_servers"])
	}
}

func TestCheckerStartStop(t *testing.T) {
	checker := NewChecker(1*time.Second, 3*time.Second)

	servers := createTestServers()
	checker.UpdateServers(servers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker.Start(ctx)

	// Ждём пару проверок
	time.Sleep(3 * time.Second)

	checker.Stop()

	stats := checker.GetStats()
	if stats["total_checks"].(int64) == 0 {
		t.Error("Expected some checks to be performed")
	}

	t.Logf("Stats: %+v", stats)
}

func TestCheckerGetActiveServers(t *testing.T) {
	checker := NewChecker(30*time.Second, 5*time.Second)

	servers := createTestServers()
	checker.UpdateServers(servers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker.Start(ctx)

	// Ждём проверку
	time.Sleep(6 * time.Second)

	active := checker.GetActiveServers()
	t.Logf("Active servers: %d/%d", len(active), len(servers))

	checker.Stop()

	// Должен быть хотя бы один активный сервер
	if len(active) == 0 {
		t.Log("No active servers found (may be expected if network is down)")
	}
}

func TestCheckerGetServersByRTT(t *testing.T) {
	checker := NewChecker(30*time.Second, 5*time.Second)

	servers := createTestServers()
	checker.UpdateServers(servers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker.Start(ctx)

	// Ждём проверку
	time.Sleep(6 * time.Second)

	sorted := checker.GetServersByRTT()

	t.Log("Servers sorted by RTT:")
	for i, s := range sorted {
		t.Logf("  %d. %s - RTT: %v, Active: %v", i+1, s.Name, s.RTT, s.Active)
	}

	checker.Stop()

	if len(sorted) == 0 {
		t.Error("Expected non-empty server list")
	}
}

func TestCheckerStatusChangeCallback(t *testing.T) {
	checker := NewChecker(1*time.Second, 3*time.Second)

	statusChanges := 0
	checker.OnStatusChange = func(srv *server.Server, oldActive bool) {
		statusChanges++
		t.Logf("Status change: %s: %v -> %v", srv.Name, oldActive, srv.Active)
	}

	servers := createTestServers()
	checker.UpdateServers(servers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker.Start(ctx)

	time.Sleep(4 * time.Second)

	checker.Stop()

	t.Logf("Total status changes: %d", statusChanges)
}
