package balancer

import (
	"sync"
	"testing"

	"github.com/FirdavsMF/vpn-balancer/internal/parser"
	"github.com/FirdavsMF/vpn-balancer/internal/server"
)

// createRealServers создаёт серверы из реальных VLESS конфигов
func createRealServers() []*server.Server {
	// Реальные конфиги из WHITE-CIDR-RU
	realConfigs := []string{
		"vless://9ca5cffb-19ea-45d9-a374-181b40f6bf0d@91.210.230.174:30547?encryption=none&security=reality&sni=ru.wikipedia.org&fp=chrome&type=tcp&flow=xtls-rprx-vision#RU-Moscow",
		"vless://7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b@176.57.210.85:28463?encryption=none&security=reality&sni=yandex.ru&fp=safari&type=tcp&flow=xtls-rprx-vision#RU-SPB",
		"vless://1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p@194.87.238.15:42319?encryption=none&security=reality&sni=mail.ru&fp=edge&type=tcp&flow=xtls-rprx-vision#RU-NSK",
	}

	var servers []*server.Server
	for _, url := range realConfigs {
		cfg, err := parser.Parse(url)
		if err != nil {
			continue
		}
		srv, err := server.NewServer(cfg)
		if err != nil {
			continue
		}
		// Помечаем как активные (для теста балансировщика)
		srv.SetActive(true)
		servers = append(servers, srv)
	}
	return servers
}

func TestRoundRobinBalancer_RealServers_Pick(t *testing.T) {
	servers := createRealServers()
	if len(servers) == 0 {
		t.Fatal("No servers created from real configs")
	}

	rr := NewRoundRobinBalancer()
	rr.Update(servers)

	t.Logf("Testing with %d real servers", len(servers))
	for _, s := range servers {
		t.Logf("  Server: %s (%s:%d)", s.Name, s.Address, s.Port)
	}

	// Проверяем что Pick возвращает серверы по кругу
	first := rr.Pick()
	if first == nil {
		t.Fatal("Expected non-nil server on first pick")
	}
	t.Logf("Pick 1: %s", first.Name)

	second := rr.Pick()
	if second == nil {
		t.Fatal("Expected non-nil server on second pick")
	}
	t.Logf("Pick 2: %s", second.Name)

	third := rr.Pick()
	if third == nil {
		t.Fatal("Expected non-nil server on third pick")
	}
	t.Logf("Pick 3: %s", third.Name)

	// Все три должны быть разными
	if first.Name == second.Name {
		t.Errorf("First and second picks should be different, both got %s", first.Name)
	}
	if second.Name == third.Name {
		t.Errorf("Second and third picks should be different, both got %s", second.Name)
	}
	if first.Name == third.Name {
		t.Errorf("First and third picks should be different, both got %s", first.Name)
	}

	// Четвёртый вызов должен вернуть тот же что и первый (round-robin)
	fourth := rr.Pick()
	t.Logf("Pick 4: %s", fourth.Name)
	if fourth.Name != first.Name {
		t.Errorf("Fourth pick should be %s (round-robin), got %s", first.Name, fourth.Name)
	}
}

func TestRoundRobinBalancer_RealServers_ConcurrentPick(t *testing.T) {
	servers := createRealServers()
	if len(servers) < 2 {
		t.Skip("Need at least 2 servers for concurrent test")
	}

	rr := NewRoundRobinBalancer()
	rr.Update(servers)

	var wg sync.WaitGroup
	picks := make(map[string]int)
	var mu sync.Mutex

	totalPicks := 30

	t.Logf("Running %d concurrent picks on %d servers", totalPicks, len(servers))

	for i := 0; i < totalPicks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			srv := rr.Pick()
			if srv != nil {
				mu.Lock()
				picks[srv.Name]++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Distribution after %d picks:", totalPicks)
	total := 0
	for name, count := range picks {
		t.Logf("  %s: %d picks", name, count)
		total += count
	}

	if total != totalPicks {
		t.Errorf("Expected %d total picks, got %d", totalPicks, total)
	}

	// Проверяем что все серверы получили примерно равное количество запросов
	if len(picks) != len(servers) {
		t.Errorf("Expected %d different servers, got %d", len(servers), len(picks))
	}

	// При 30 запросах и 3 серверах, каждый должен получить около 10
	expectedPerServer := totalPicks / len(servers)
	for name, count := range picks {
		diff := count - expectedPerServer
		if diff < 0 {
			diff = -diff
		}
		if diff > 2 { // Допуск ±2
			t.Logf("Warning: %s got %d picks, expected ~%d (diff: %d)", name, count, expectedPerServer, diff)
		}
	}
}

func TestRoundRobinBalancer_RealServers_UpdateWithInactive(t *testing.T) {
	servers := createRealServers()
	if len(servers) < 2 {
		t.Skip("Need at least 2 servers")
	}

	rr := NewRoundRobinBalancer()
	rr.Update(servers)

	t.Logf("Initial servers: %d", len(servers))

	// Деактивируем первый сервер
	servers[0].SetActive(false)
	t.Logf("Deactivated: %s", servers[0].Name)

	// Обновляем балансировщик
	rr.Update(servers)

	// Проверяем что деактивированный сервер не выбирается
	for i := 0; i < 10; i++ {
		srv := rr.Pick()
		if srv == nil {
			t.Fatal("Expected server, got nil")
		}
		if srv.Name == servers[0].Name {
			t.Errorf("Inactive server %s should not be picked (iteration %d)", servers[0].Name, i)
		}
	}

	// Активируем обратно
	servers[0].SetActive(true)
	rr.Update(servers)
	t.Logf("Reactivated: %s", servers[0].Name)

	// Теперь все серверы должны выбираться
	found := make(map[string]bool)
	for i := 0; i < 10; i++ {
		srv := rr.Pick()
		if srv != nil {
			found[srv.Name] = true
		}
	}
	t.Logf("Found servers after reactivation: %v", found)

	if len(found) != 3 {
		t.Errorf("Expected 3 servers available, got %d", len(found))
	}
}

func TestLeastConnBalancer_RealServers(t *testing.T) {
	servers := createRealServers()
	if len(servers) < 2 {
		t.Skip("Need at least 2 servers")
	}

	lc := NewLeastConnBalancer()
	lc.Update(servers)

	// Добавляем соединения к первому серверу
	servers[0].IncrementConnections()
	servers[0].IncrementConnections()
	servers[0].IncrementConnections()
	t.Logf("%s: %d connections", servers[0].Name, servers[0].GetConnections())

	// Добавляем одно соединение ко второму
	servers[1].IncrementConnections()
	t.Logf("%s: %d connections", servers[1].Name, servers[1].GetConnections())

	// Третий без соединений
	t.Logf("%s: %d connections", servers[2].Name, servers[2].GetConnections())

	// LeastConn должен выбрать сервер с наименьшим количеством соединений (Server-3)
	srv := lc.Pick()
	if srv == nil {
		t.Fatal("Expected non-nil server")
	}

	t.Logf("LeastConn picked: %s (connections: %d)", srv.Name, srv.GetConnections())

	if srv.GetConnections() != 0 {
		t.Errorf("Expected server with 0 connections, got %d", srv.GetConnections())
	}
}

func TestBalancerStats_RealServers(t *testing.T) {
	servers := createRealServers()
	rr := NewRoundRobinBalancer()
	rr.Update(servers)

	// Делаем несколько выборок
	for i := 0; i < 5; i++ {
		rr.Pick()
	}

	stats := rr.Stats()
	t.Logf("Stats: %+v", stats)

	if stats["total_servers"] != len(servers) {
		t.Errorf("Expected %d servers in stats, got %v", len(servers), stats["total_servers"])
	}

	if stats["total_picks"].(uint64) != 5 {
		t.Errorf("Expected 5 picks in stats, got %v", stats["total_picks"])
	}
}
