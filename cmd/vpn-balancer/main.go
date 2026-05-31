package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/FirdavsMF/vpn-balancer/internal/downloader"
	"github.com/FirdavsMF/vpn-balancer/internal/parser"
	"github.com/FirdavsMF/vpn-balancer/internal/server"
)

func main() {
	fmt.Println("=== VPN Balancer v0.1.0 ===")
	fmt.Println("Author: FirdavsMF")
	fmt.Println()

	// Источники конфигов
	sources := []string{
		"https://raw.githack.com/igareck/vpn-configs-for-russia/main/WHITE-CIDR-RU-all.txt",
	}

	fmt.Println("Fetching VLESS configs from sources...")

	// Загружаем конфиги
	urls, err := downloader.FetchAll(sources)
	if err != nil {
		fmt.Printf("Error fetching configs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Downloaded %d lines\n", len(urls))

	// Парсим и создаём серверы
	var servers []*server.Server
	successCount := 0
	failCount := 0

	for i, url := range urls {
		// Пропускаем строки, которые не являются VLESS URL
		if !strings.HasPrefix(url, "vless://") {
			continue
		}

		config, err := parser.Parse(url)
		if err != nil {
			fmt.Printf("[%d] Parse error: %v\n", i+1, err)
			failCount++
			continue
		}

		srv := server.NewServer(config)
		servers = append(servers, srv)
		successCount++

		fmt.Printf("[%d] %s\n", successCount, srv)
	}

	fmt.Println()
	fmt.Printf("=== Summary ===\n")
	fmt.Printf("Total lines: %d\n", len(urls))
	fmt.Printf("Successfully parsed: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failCount)
	fmt.Printf("Total servers: %d\n", len(servers))

	// Показываем статистику по типам
	stats := make(map[string]int)
	for _, srv := range servers {
		stats[srv.Network]++
		stats["security_"+srv.Security]++
	}

	fmt.Println("\nNetwork types:")
	for network, count := range stats {
		if !strings.HasPrefix(network, "security_") {
			fmt.Printf("  %s: %d\n", network, count)
		}
	}

	fmt.Println("\nSecurity types:")
	for security, count := range stats {
		if strings.HasPrefix(security, "security_") {
			fmt.Printf("  %s: %d\n", strings.TrimPrefix(security, "security_"), count)
		}
	}
}
