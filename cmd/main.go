package main

import (
	"log"
	"router/internal/config"
	"router/internal/panel"
	"router/internal/proxy"
	"router/internal/storage"
)

func main() {
	cfg := config.Load()

	// Хранилище правил
	rules := storage.NewRuleStore()
	rules.Add("avayusstroi.by", "localhost:8080")
	rules.Add("avayusstroi.xyz", "localhost:8090")

	// Запускаем панель управления
	go func() {
		if err := panel.StartPanel(":8162", cfg, rules); err != nil {
			log.Fatalf("Ошибка запуска панели: %v", err)
		}
	}()

	// Запускаем сам прокси
	proxy.StartProxy(rules)
}
