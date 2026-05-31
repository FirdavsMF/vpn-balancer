package main

import (
"context"
"fmt"
"os"
"os/signal"
"strings"
"syscall"
"time"

"github.com/FirdavsMF/vpn-balancer/internal/downloader"
"github.com/FirdavsMF/vpn-balancer/internal/health"
"github.com/FirdavsMF/vpn-balancer/internal/parser"
"github.com/FirdavsMF/vpn-balancer/internal/server"
)

func main() {
fmt.Println("=== VPN Balancer v0.3.1 ===")
fmt.Println("Author: FirdavsMF")

sources := []string{
"https://raw.githack.com/igareck/vpn-configs-for-russia/main/WHITE-CIDR-RU-all.txt",
}

fmt.Println("\nFetching VLESS configs...")
urls, err := downloader.FetchAll(sources)
if err != nil {
fmt.Printf("Warning: Could not fetch remote configs: %v\n", err)
fmt.Println("Using local test configs...")
urls = getTestConfigs()
}

fmt.Printf("Downloaded %d lines\n", len(urls))

var servers []*server.Server
successCount := 0

for _, url := range urls {
if !strings.HasPrefix(url, "vless://") {
continue
}
config, err := parser.Parse(url)
if err != nil {
continue
}
srv, err := server.NewServer(config)
if err != nil {
continue
}
servers = append(servers, srv)
successCount++
}

fmt.Printf("Successfully created %d servers\n", len(servers))

if len(servers) == 0 {
fmt.Println("No servers available, exiting")
os.Exit(1)
}

checker := health.NewChecker(30*time.Second, 10*time.Second)
checker.OnStatusChange = func(srv *server.Server, oldActive bool) {
if srv.IsActive() {
fmt.Printf("✅ Server UP: %s (RTT: %v)\n", srv.Name, srv.GetRTT())
} else {
fmt.Printf("❌ Server DOWN: %s\n", srv.Name)
}
}
checker.UpdateServers(servers)

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

checker.Start(ctx)

fmt.Println("\nWaiting for initial health check (30 seconds)...")
fmt.Println("Press Ctrl+C to stop")

go func() {
time.Sleep(35 * time.Second)
printStats(checker)
}()

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

ticker := time.NewTicker(1 * time.Minute)
defer ticker.Stop()

for {
select {
case <-sigCh:
fmt.Println("\nShutting down...")
checker.Stop()
printStats(checker)
return
case <-ticker.C:
printStats(checker)
}
}
}

func printStats(checker *health.Checker) {
stats := checker.GetStats()
fmt.Println("\n=== Server Statistics ===")
fmt.Printf("Active: %v/%v\n", stats["active_servers"], stats["total_servers"])
fmt.Printf("Total checks: %v\n", stats["total_checks"])
fmt.Printf("Failed checks: %v\n", stats["failed_checks"])

fmt.Println("\nTop servers by RTT:")
sorted := checker.GetServersByRTT()
count := 0
for _, s := range sorted {
if s.IsActive() && count < 5 {
fmt.Printf("  %d. %s (RTT: %v, connections: %d)\n",
count+1, s.Name, s.GetRTT(), s.GetConnections())
count++
}
}
fmt.Println()
}

func getTestConfigs() []string {
return []string{
"vless://9ca5cffb-19ea-45d9-a374-181b40f6bf0d@91.210.230.174:30547?encryption=none&security=reality&sni=ru.wikipedia.org&fp=chrome&type=tcp&flow=xtls-rprx-vision#RU-Moscow",
"vless://7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b@176.57.210.85:28463?encryption=none&security=reality&sni=yandex.ru&fp=safari&type=tcp&flow=xtls-rprx-vision#RU-SPB",
"vless://1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p@194.87.238.15:42319?encryption=none&security=reality&sni=mail.ru&fp=edge&type=tcp&flow=xtls-rprx-vision#RU-NSK",
}
}
