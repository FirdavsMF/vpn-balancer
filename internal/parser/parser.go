package parser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// VLESSConfig представляет распарсенную конфигурацию VLESS
type VLESSConfig struct {
	Address string
	Port    int
	UUID    string

	Encryption string
	Flow       string

	Security    string
	SNI         string
	Fingerprint string
	SID         string
	PBK         string
	SPX         string

	Network     string
	Path        string
	Host        string
	ServiceName string

	AllowInsecure bool
	Name          string
}

// Parse парсит VLESS URL и возвращает структуру VLESSConfig
func Parse(vlessURL string) (*VLESSConfig, error) {
	if !strings.HasPrefix(vlessURL, "vless://") {
		return nil, fmt.Errorf("invalid VLESS URL: must start with vless://")
	}

	urlStr := strings.TrimPrefix(vlessURL, "vless://")

	// Извлекаем фрагмент (имя)
	var name string
	if idx := strings.LastIndex(urlStr, "#"); idx != -1 {
		name = urlStr[idx+1:]
		urlStr = urlStr[:idx]
	}

	// Разделяем UUID и остальную часть по @
	parts := strings.SplitN(urlStr, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid VLESS URL: missing @ separator")
	}

	uuid := parts[0]
	hostPart := parts[1]

	// Разделяем хост:порт и параметры
	var hostPort, params string
	if idx := strings.Index(hostPart, "?"); idx != -1 {
		hostPort = hostPart[:idx]
		params = hostPart[idx+1:]
	} else {
		hostPort = hostPart
	}

	// Убираем trailing slash и всё что после него до ?
	hostPort = strings.TrimRight(hostPort, "/")

	// Парсим хост и порт
	host, portStr, err := parseHostPort(hostPort)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host:port: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// Создаём базовую конфигурацию
	config := &VLESSConfig{
		Address:    host,
		Port:       port,
		UUID:       uuid,
		Encryption: "none",
		Network:    "tcp",
		Security:   "none",
	}

	// Парсим параметры
	if params != "" {
		if err := parseParams(params, config); err != nil {
			return nil, fmt.Errorf("failed to parse params: %w", err)
		}
	}

	// Устанавливаем имя
	if name != "" {
		decodedName, err := url.QueryUnescape(name)
		if err == nil {
			config.Name = decodedName
		} else {
			config.Name = name
		}
	} else {
		config.Name = fmt.Sprintf("%s:%d", config.Address, config.Port)
	}

	return config, nil
}

func parseHostPort(hostPort string) (string, string, error) {
	// Очищаем от возможных путей
	hostPort = strings.SplitN(hostPort, "/", 2)[0]

	// Проверяем IPv6 адрес в квадратных скобках
	if strings.HasPrefix(hostPort, "[") {
		closeBracket := strings.Index(hostPort, "]")
		if closeBracket == -1 {
			return "", "", fmt.Errorf("invalid IPv6 address format")
		}

		host := hostPort[1:closeBracket]

		if len(hostPort) > closeBracket+1 {
			if hostPort[closeBracket+1] == ':' {
				port := hostPort[closeBracket+2:]
				return host, port, nil
			}
		}

		return "", "", fmt.Errorf("port not specified for IPv6 address")
	}

	// Обычный хост:порт
	parts := strings.SplitN(hostPort, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid host:port format: %s", hostPort)
	}

	return parts[0], parts[1], nil
}

func parseParams(paramsStr string, config *VLESSConfig) error {
	values, err := url.ParseQuery(paramsStr)
	if err != nil {
		return fmt.Errorf("failed to parse query params: %w", err)
	}

	// Основные параметры
	if enc := values.Get("encryption"); enc != "" {
		config.Encryption = enc
	}

	if flow := values.Get("flow"); flow != "" {
		config.Flow = flow
	}

	// Security параметры
	if security := values.Get("security"); security != "" {
		config.Security = security
	}

	if sni := values.Get("sni"); sni != "" {
		config.SNI = sni
	}

	if fp := values.Get("fp"); fp != "" {
		config.Fingerprint = fp
	}

	// Reality параметры
	if sid := values.Get("sid"); sid != "" {
		config.SID = sid
	}

	if pbk := values.Get("pbk"); pbk != "" {
		config.PBK = pbk
	}

	if spx := values.Get("spx"); spx != "" {
		config.SPX = spx
	}

	// Транспорт
	if type_ := values.Get("type"); type_ != "" {
		config.Network = type_
	}

	if path := values.Get("path"); path != "" {
		config.Path = path
	}

	if host := values.Get("host"); host != "" {
		config.Host = host
	}

	if serviceName := values.Get("serviceName"); serviceName != "" {
		config.ServiceName = serviceName
	}

	// Дополнительно
	if allowInsecure := values.Get("allowInsecure"); allowInsecure == "1" || allowInsecure == "true" {
		config.AllowInsecure = true
	}

	return nil
}

// String возвращает строковое представление конфигурации
func (c *VLESSConfig) String() string {
	uuidPrefix := c.UUID
	if len(uuidPrefix) > 8 {
		uuidPrefix = uuidPrefix[:8]
	}

	return fmt.Sprintf(
		"VLESS[%s...] %s:%d (security=%s, network=%s, name=%s)",
		uuidPrefix,
		c.Address,
		c.Port,
		c.Security,
		c.Network,
		c.Name,
	)
}
