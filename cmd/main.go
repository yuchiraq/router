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
	storage := storage.NewStorage("storage.json")
	rules := storage.NewRuleStore(storage)

	// Запускаем панель управления
	go func() {
		if err := panel.StartPanel(":8162", cfg, rules); err != nil {
			log.Fatalf("Ошибка запуска панели: %v", err)
		}
	}()

	// Запускаем сам прокси
	proxy.StartProxy(rules)
}
