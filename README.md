# VPN Balancer

[![CI/CD](https://github.com/FirdavsMF/vpn-balancer/actions/workflows/ci.yml/badge.svg)](https://github.com/FirdavsMF/vpn-balancer/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Балансировщик VPN-соединений с поддержкой VLESS протокола.

## Особенности

- Поддержка VLESS протокола
- Балансировка нагрузки между несколькими серверами
- Проверка здоровья серверов
- Автоматическая загрузка конфигураций
- Ротация логов

## Установка

### Из исходного кода

```bash
git clone https://github.com/FirdavsMF/vpn-balancer.git
cd vpn-balancer
make build
