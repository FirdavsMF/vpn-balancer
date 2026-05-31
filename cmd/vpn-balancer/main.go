package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FirdavsMF/vpn-balancer/internal/downloader"
	"github.com/FirdavsMF/vpn-balancer/internal/parser"
	"github.com/FirdavsMF/vpn-balancer/internal/server"
)

func main() {
	fmt.Println("=== VPN Balancer v0.2.0 ===")
	fmt.Println("Author: FirdavsMF")
	fmt.Println()

	// Загружаем конфиги
	sources := []string{
		"https://raw.githack.com/igareck/vpn-configs-for-russia/main/WHITE-CIDR-RU-all.txt",
	}

	fmt.Println("Fetching VLESS configs...")
	urls, err := downloader.FetchAll(sources)
	if err != nil {
		fmt.Printf("Error fetching configs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Downloaded %d lines\n", len(urls))

	// Парсим и создаём серверы
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

	// Тестируем подключение к первому серверу
	if len(servers) > 0 {
		fmt.Println("\n=== Testing VLESS Connection ===")
		testServer := servers[0]
		fmt.Printf("Testing server: %s\n", testServer)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		start := time.Now()
		conn, err := testServer.Connect(ctx, "8.8.8.8:53")
		if err != nil {
			fmt.Printf("Connection failed: %v\n", err)
		} else {
			fmt.Printf("Connected successfully! RTT: %v\n", time.Since(start))
			conn.Close()
		}
	}

	// Статистика
	fmt.Println("\n=== Server Statistics ===")
	stats := make(map[string]int)
	for _, srv := range servers {
		stats[srv.Network]++
		stats["security_"+srv.Security]++
	}

	fmt.Println("Network types:")
	for k, v := range stats {
		if !strings.HasPrefix(k, "security_") {
			fmt.Printf("  %s: %d\n", k, v)
		}
	}

	fmt.Println("\nSecurity types:")
	for k, v := range stats {
		if strings.HasPrefix(k, "security_") {
			fmt.Printf("  %s: %d\n", strings.TrimPrefix(k, "security_"), v)
		}
	}
}
