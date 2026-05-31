package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/balancer"
	"github.com/FirdavsMF/vpn-balancer/internal/downloader"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║     VPN Balancer v0.5.0             ║")
	fmt.Println("║     Author: FirdavsMF               ║")
	fmt.Println("║     Round Robin + SOCKS5            ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// Создаём менеджер
	manager := balancer.NewManager()

	// Источники конфигов
	sources := []string{
		"https://raw.githack.com/igareck/vpn-configs-for-russia/main/WHITE-CIDR-RU-all.txt",
	}

	fmt.Println("Fetching VLESS configs...")
	urls, err := downloader.FetchAll(sources)
	if err != nil {
		log.Printf("Warning: Could not fetch remote configs: %v", err)
		fmt.Println("Using local test configs...")
		urls = getTestConfigs()
	}

	fmt.Printf("Downloaded %d lines\n", len(urls))

	// Добавляем серверы
	if err := manager.AddServersFromURLs(urls); err != nil {
		log.Fatalf("Failed to add servers: %v", err)
	}

	// Запускаем health checker (каждые 60 секунд)
	manager.StartHealthChecker(60*time.Second, 10*time.Second)

	fmt.Println("\nWaiting for initial health check...")
	time.Sleep(15 * time.Second)

	// Запускаем SOCKS5 прокси
	proxyAddr := ":1080"
	if err := manager.StartProxy(proxyAddr); err != nil {
		log.Fatalf("Failed to start SOCKS5 proxy: %v", err)
	}

	fmt.Printf("\n✅ SOCKS5 proxy started on %s\n", proxyAddr)
	fmt.Println("Configure your browser: SOCKS5 localhost:1080")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Периодический вывод статистики
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			printStats(manager)
		}
	}()

	// Ожидаем сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	manager.Stop()
	printStats(manager)
	fmt.Println("Goodbye!")
}

func printStats(manager *balancer.Manager) {
	stats := manager.GetStats()
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║        Server Statistics            ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Printf("Active servers: %v/%v\n", stats["active_servers"], stats["total_servers"])
	fmt.Printf("Balancer type: %v\n", stats["balancer_type"])
	fmt.Printf("Total health checks: %v\n", stats["total_checks"])
	fmt.Printf("Failed health checks: %v\n", stats["failed_checks"])
	fmt.Println()
}

func getTestConfigs() []string {
	return []string{
		"vless://9ca5cffb-19ea-45d9-a374-181b40f6bf0d@91.210.230.174:30547?encryption=none&security=reality&sni=ru.wikipedia.org&fp=chrome&type=tcp&flow=xtls-rprx-vision#RU-Moscow",
		"vless://7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b@176.57.210.85:28463?encryption=none&security=reality&sni=yandex.ru&fp=safari&type=tcp&flow=xtls-rprx-vision#RU-SPB",
	}
}
