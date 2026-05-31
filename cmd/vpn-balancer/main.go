package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/balancer"
	"github.com/FirdavsMF/vpn-balancer/internal/watcher"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║     VPN Balancer v0.7.0             ║")
	fmt.Println("║     Author: FirdavsMF               ║")
	fmt.Println("║     Auto-Reload + RR + Graceful     ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	ctx := context.Background()
	manager := balancer.NewManager()

	sources := []string{
		"https://raw.githack.com/igareck/vpn-configs-for-russia/main/WHITE-CIDR-RU-all.txt",
	}

	fmt.Println("Fetching VLESS configs...")
	if err := manager.AddServersFromURLs(sources); err != nil {
		log.Fatalf("Failed to add servers: %v", err)
	}

	// Запускаем health checker
	manager.StartHealthChecker(60*time.Second, 10*time.Second)

	// Создаём и запускаем Reloader (каждые 5 минут)
	reloader := watcher.NewReloader(5*time.Minute, sources, manager.Reload)
	reloader.Start(ctx)

	fmt.Println("\nWaiting for initial health check...")
	time.Sleep(15 * time.Second)

	// Запускаем SOCKS5 прокси
	proxyAddr := ":1080"
	if err := manager.StartProxy(proxyAddr); err != nil {
		log.Fatalf("Failed to start SOCKS5 proxy: %v", err)
	}

	fmt.Printf("\n✅ SOCKS5 proxy started on %s\n", proxyAddr)
	fmt.Println("Configure your browser: SOCKS5 localhost:1080")
	fmt.Println("Configs auto-reload every 5 minutes")
	fmt.Println("Press Ctrl+C for graceful shutdown")
	fmt.Println("Send SIGHUP for manual reload: kill -HUP <pid>")
	fmt.Println()

	// Периодический вывод статистики
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			printStats(manager)
		}
	}()

	// Обработка сигналов
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP:
			fmt.Println("\n📡 Received SIGHUP - manual reload...")
			if err := reloader.ReloadNow(); err != nil {
				log.Printf("Manual reload failed: %v", err)
			} else {
				fmt.Println("✅ Manual reload completed")
				printStats(manager)
			}

		case syscall.SIGINT, syscall.SIGTERM:
			fmt.Printf("\n📡 Received signal: %v\n", sig)
			fmt.Println("Starting graceful shutdown...")

			reloader.Stop()
			manager.Shutdown()

			printStats(manager)
			fmt.Println("Goodbye!")
			return
		}
	}
}

func printStats(manager *balancer.Manager) {
	stats := manager.GetStats()
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║        Server Statistics            ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Printf("Active servers: %v/%v\n", stats["active_servers"], stats["total_servers"])
	fmt.Printf("Loaded servers: %v\n", stats["total_servers_loaded"])
	fmt.Printf("Balancer type: %v\n", stats["balancer_type"])
	fmt.Printf("Proxy connections: %v active / %v total\n",
		stats["proxy_active_connections"], stats["proxy_total_connections"])
	fmt.Printf("Health checks: %v total / %v failed\n",
		stats["total_checks"], stats["failed_checks"])
	fmt.Println()
}
