package parser

import (
	"testing"
)

func TestParseRealServer(t *testing.T) {
	url := "vless://9ca5cffb-19ea-45d9-a374-181b40f6bf0d@91.210.230.174:30547?encryption=none&security=reality&sni=ru.wikipedia.org&fp=chrome&pbk=47xJcVnJq9tGqZrZqGXwLjXfTqXpXbYqZrZqGXwLjXfTqXpXbYqZrZqGXwLjXfTqXpXbYqZrZqGXwLjXfTqXpXbY&sid=6f3e2d1c&spx=%2F&type=tcp&flow=xtls-rprx-vision#RU-Server-Moscow"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.UUID != "9ca5cffb-19ea-45d9-a374-181b40f6bf0d" {
		t.Errorf("Expected UUID '9ca5cffb-19ea-45d9-a374-181b40f6bf0d', got '%s'", config.UUID)
	}

	if config.Address != "91.210.230.174" {
		t.Errorf("Expected address '91.210.230.174', got '%s'", config.Address)
	}

	if config.Port != 30547 {
		t.Errorf("Expected port 30547, got %d", config.Port)
	}

	if config.Name != "RU-Server-Moscow" {
		t.Errorf("Expected name 'RU-Server-Moscow', got '%s'", config.Name)
	}

	if config.Security != "reality" {
		t.Errorf("Expected security 'reality', got '%s'", config.Security)
	}

	if config.SNI != "ru.wikipedia.org" {
		t.Errorf("Expected SNI 'ru.wikipedia.org', got '%s'", config.SNI)
	}

	if config.Fingerprint != "chrome" {
		t.Errorf("Expected fingerprint 'chrome', got '%s'", config.Fingerprint)
	}

	if config.Flow != "xtls-rprx-vision" {
		t.Errorf("Expected flow 'xtls-rprx-vision', got '%s'", config.Flow)
	}
}

func TestParseMultipleRealServers(t *testing.T) {
	servers := []struct {
		name string
		url  string
	}{
		{
			name: "Server 1 - Saint Petersburg",
			url:  "vless://7f3e5a1b-2c4d-4e5f-8a9b-1c2d3e4f5a6b@176.57.210.85:28463?encryption=none&security=reality&sni=yandex.ru&fp=safari&pbk=TrQxWvYmLpNqJtRzUxVbNcDfGhJkLzXcVbNmQwErTy&sid=8g7h6j5k&type=tcp&flow=xtls-rprx-vision#RU-Server-SPB",
		},
		{
			name: "Server 2 - Novosibirsk",
			url:  "vless://1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p@194.87.238.15:42319?encryption=none&security=reality&sni=mail.ru&fp=edge&pbk=YwUxIoPkLjHgFdSaQwErTyZuXcVbNmQwErTyZuXcVbNm&sid=9h0i1j2k&type=tcp&flow=xtls-rprx-vision#RU-Server-NSK",
		},
	}

	for _, server := range servers {
		t.Run(server.name, func(t *testing.T) {
			config, err := Parse(server.url)
			if err != nil {
				t.Fatalf("Parse failed for %s: %v", server.name, err)
			}

			if config.Security != "reality" {
				t.Errorf("Server %s: expected security 'reality', got '%s'", server.name, config.Security)
			}

			if config.Flow != "xtls-rprx-vision" {
				t.Errorf("Server %s: expected flow 'xtls-rprx-vision', got '%s'", server.name, config.Flow)
			}

			if config.Network != "tcp" {
				t.Errorf("Server %s: expected network 'tcp', got '%s'", server.name, config.Network)
			}
		})
	}
}

func TestParseRealityWithAllParams(t *testing.T) {
	url := "vless://9ca5cffb-19ea-45d9-a374-181b40f6bf0d@91.210.230.174:30547?" +
		"encryption=none&security=reality&sni=ru.wikipedia.org&fp=chrome&" +
		"pbk=47xJcVnJq9tGqZrZqGXwLjXfTqXpXbYqZrZqGXwLjXfTqXpXbYqZrZqGXwLjXfTqXpXbYqZrZqGXwLjXfTqXpXbY&" +
		"sid=6f3e2d1c&spx=%2F&type=tcp&flow=xtls-rprx-vision#RU-Server-Moscow"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.Port != 30547 {
		t.Errorf("Expected port 30547, got %d", config.Port)
	}

	if config.Security != "reality" {
		t.Errorf("Expected security 'reality', got '%s'", config.Security)
	}

	if config.SNI != "ru.wikipedia.org" {
		t.Errorf("Expected SNI 'ru.wikipedia.org', got '%s'", config.SNI)
	}

	if config.Fingerprint != "chrome" {
		t.Errorf("Expected fingerprint 'chrome', got '%s'", config.Fingerprint)
	}

	if config.Flow != "xtls-rprx-vision" {
		t.Errorf("Expected flow 'xtls-rprx-vision', got '%s'", config.Flow)
	}

	if config.Name != "RU-Server-Moscow" {
		t.Errorf("Expected name 'RU-Server-Moscow', got '%s'", config.Name)
	}
}

func TestParseWebSocketRealServer(t *testing.T) {
	url := "vless://2b4d6f8h-1a3c5e7g-9i0k2m4o-6q8s0u2w@185.130.105.78:5443?" +
		"encryption=none&security=tls&sni=cloudflare.com&fp=random&" +
		"type=ws&path=/vless&host=api.example.com&allowInsecure=0#RU-WS-Server"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.Network != "ws" {
		t.Errorf("Expected network 'ws', got '%s'", config.Network)
	}

	if config.Path != "/vless" {
		t.Errorf("Expected path '/vless', got '%s'", config.Path)
	}

	if config.Host != "api.example.com" {
		t.Errorf("Expected host 'api.example.com', got '%s'", config.Host)
	}
}

func TestParseGRPCRealServer(t *testing.T) {
	url := "vless://3c5e7g9i-2d4f6h8j-0k2m4o6q-8s0u2w4y@95.216.45.123:1234?" +
		"encryption=none&security=tls&sni=grpc.example.com&fp=firefox&" +
		"type=grpc&serviceName=vless-service#RU-GRPC-Server"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.Network != "grpc" {
		t.Errorf("Expected network 'grpc', got '%s'", config.Network)
	}

	if config.ServiceName != "vless-service" {
		t.Errorf("Expected serviceName 'vless-service', got '%s'", config.ServiceName)
	}

	if config.Security != "tls" {
		t.Errorf("Expected security 'tls', got '%s'", config.Security)
	}
}

func TestParseInvalidURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"No vless prefix", "invalid://uuid@host:port"},
		{"No @ separator", "vless://uuid-without-at-host:port"},
		{"Missing port", "vless://9ca5cffb-19ea-45d9-a374-181b40f6bf0d@91.210.230.174#NoPort"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.url)
			if err == nil {
				t.Errorf("Expected error for invalid URL: %s", test.url)
			}
		})
	}
}

func TestParseWithoutName(t *testing.T) {
	url := "vless://9ca5cffb-19ea-45d9-a374-181b40f6bf0d@91.210.230.174:30547"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	expectedName := "91.210.230.174:30547"
	if config.Name != expectedName {
		t.Errorf("Expected auto-generated name '%s', got '%s'", expectedName, config.Name)
	}
}

func TestParseRealityWithSIDAndPBK(t *testing.T) {
	url := "vless://4d6f8h0j-3e5g7i9k-1m3o5q7s-9u1w3y5a@176.57.210.85:28463?" +
		"security=reality&pbk=TrQxWvYmLpNqJtRzUxVbNcDfGhJkLzXcVbNmQwErTy&" +
		"sid=8g7h6j5k&sni=yandex.ru&fp=safari&flow=xtls-rprx-vision#RU-Reality-Test"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.Security != "reality" {
		t.Errorf("Expected security 'reality', got '%s'", config.Security)
	}

	if config.SNI != "yandex.ru" {
		t.Errorf("Expected SNI 'yandex.ru', got '%s'", config.SNI)
	}

	if config.Fingerprint != "safari" {
		t.Errorf("Expected fingerprint 'safari', got '%s'", config.Fingerprint)
	}

	if config.Flow != "xtls-rprx-vision" {
		t.Errorf("Expected flow 'xtls-rprx-vision', got '%s'", config.Flow)
	}
}

func TestParseIPv6(t *testing.T) {
	url := "vless://12345678-1234-1234-1234-123456789abc@[2001:db8::1]:4433#IPv6Server"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.Address != "2001:db8::1" {
		t.Errorf("Expected IPv6 address '2001:db8::1', got '%s'", config.Address)
	}

	if config.Port != 4433 {
		t.Errorf("Expected port 4433, got %d", config.Port)
	}
}

func TestParseAllowInsecureTrue(t *testing.T) {
	url := "vless://uuid@server.com:443?allowInsecure=1#InsecureServer"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if !config.AllowInsecure {
		t.Error("Expected AllowInsecure to be true")
	}
}

func TestParseAllowInsecureFalse(t *testing.T) {
	url := "vless://uuid@server.com:443?allowInsecure=0#SecureServer"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.AllowInsecure {
		t.Error("Expected AllowInsecure to be false")
	}
}

func TestParsePortWithSlash(t *testing.T) {
	// Тест для случая "443/" - порт с trailing slash
	url := "vless://d0b60427-cfa8-4680-be74-69e3a7f8e0bc@nl-5.srv.vpnza300.org:443/?security=reality&sni=nl-5.srv.vpnza300.org&fp=chrome&type=tcp&flow=xtls-rprx-vision#Test-Server"

	config, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.Port != 443 {
		t.Errorf("Expected port 443, got %d", config.Port)
	}

	if config.Address != "nl-5.srv.vpnza300.org" {
		t.Errorf("Expected address 'nl-5.srv.vpnza300.org', got '%s'", config.Address)
	}
}
