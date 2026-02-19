package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"router/internal/clog"
	"router/internal/gpt"
	"router/internal/logstream"
	"router/internal/notify"
	"router/internal/panel"
	"router/internal/proxy"
	"router/internal/stats"
	"router/internal/storage"

	"golang.org/x/crypto/acme/autocert"
)

func main() {
	// Initialize storage
	fileStorage := storage.NewStorage("rules.json")
	store := storage.NewRuleStore(fileStorage)

	// Initialize stats
	stats := stats.New()
	stats.RecordMemory()
	stats.RecordCPU()
	stats.RecordDisks()
	stats.RecordSSHConnections()

	// Initialize log broadcaster
	broadcaster := logstream.New()
	log.SetOutput(logstream.NewConsoleMux(os.Stderr, broadcaster))

	// Suspicious IP reputation storage
	ipReputation := storage.NewIPReputationStore("ip_reputation.json")
	backupStore := storage.NewBackupStore("backup_config.json")
	notifyStore := storage.NewNotificationStore("notifications.json")
	gptStore := storage.NewGPTStore("gpt.json")
	gptClient := gpt.NewClient(gptStore)
	notifier := notify.NewTelegramNotifier(notifyStore)
	backupStore.OnResult = func(err error, archivePath string) {
		if err != nil {
			notifier.Notify("backup_failure", "backup-failure", "❌ Backup failed\n"+err.Error())
			return
		}
		notifier.Notify("backup_success", "backup-success:"+archivePath, "✅ Backup completed\narchive: "+archivePath)
	}
	go backupStore.Start()

	// Start memory recording
	go func() {
		for {
			stats.RecordMemory()
			stats.RecordCPU()
			stats.RecordDisks()
			stats.RecordSSHConnections()
			time.Sleep(5 * time.Second)
		}
	}()

	// Admin panel hardening defaults for internet-facing deployments.
	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASS")
	if adminUser == "" {
		adminUser = "testuser"
		clog.Warnf("ADMIN_USER is not set, using default admin user")
	}
	if adminPass == "" {
		adminPass = "testpass"
		clog.Warnf("ADMIN_PASS is not set, using default admin password")
	}
	panelAddr := strings.TrimSpace(os.Getenv("PANEL_ADDR"))
	if panelAddr == "" {
		panelAddr = "127.0.0.1:8162"
	}
	adminStore := storage.NewAdminStore("admin.json", adminUser, adminPass)

	// --- Admin Panel ---
	go func() {
		panelMux := http.NewServeMux()
		panelHandler := panel.NewHandler(store, adminStore, stats, broadcaster, ipReputation, backupStore, notifyStore, gptStore, gptClient, notifier)

		// Serve static files
		staticFS := http.FileServer(http.Dir("internal/panel/static"))
		panelMux.Handle("/static/", http.StripPrefix("/static/", staticFS))

		panelMux.HandleFunc("/", panelHandler.Index)
		panelMux.HandleFunc("/login", panelHandler.Login)
		panelMux.HandleFunc("/logout", panelHandler.Logout)
		panelMux.HandleFunc("/account", panelHandler.Account)
		panelMux.HandleFunc("/account/data", panelHandler.AccountData)
		panelMux.HandleFunc("/account/config", panelHandler.SaveAccountConfig)
		panelMux.HandleFunc("/stats", panelHandler.Stats)
		panelMux.HandleFunc("/backups", panelHandler.Backups)
		panelMux.HandleFunc("/notifications", panelHandler.Notifications)
		panelMux.HandleFunc("/settings", panelHandler.Settings)
		panelMux.HandleFunc("/stats/data", panelHandler.StatsData)
		panelMux.HandleFunc("/backups/data", panelHandler.BackupsData)
		panelMux.HandleFunc("/backups/config", panelHandler.SaveBackupsConfig)
		panelMux.HandleFunc("/backups/delete", panelHandler.DeleteBackupJob)
		panelMux.HandleFunc("/backups/run", panelHandler.RunBackupNow)
		panelMux.HandleFunc("/notifications/data", panelHandler.NotificationsData)
		panelMux.HandleFunc("/notifications/config", panelHandler.SaveNotificationsConfig)
		panelMux.HandleFunc("/notifications/test", panelHandler.TestNotification)
		panelMux.HandleFunc("/settings/data", panelHandler.SettingsData)
		panelMux.HandleFunc("/settings/config", panelHandler.SaveSettingsConfig)
		panelMux.HandleFunc("/telegram/webhook", panelHandler.TelegramWebhook)
		panelMux.HandleFunc("/stats/ban", panelHandler.BanSuspiciousIP)
		panelMux.HandleFunc("/stats/unban", panelHandler.UnbanSuspiciousIP)
		panelMux.HandleFunc("/stats/remove", panelHandler.RemoveSuspiciousIP)
		panelMux.HandleFunc("/ws/logs", panelHandler.Logs)
		panelMux.HandleFunc("/add", panelHandler.AddRule)
		panelMux.HandleFunc("/rule/maintenance", panelHandler.RuleMaintenance)
		panelMux.HandleFunc("/remove", panelHandler.RemoveRule)
		clog.Infof("Starting admin panel on %s", panelAddr)
		if err := http.ListenAndServe(panelAddr, panelMux); err != nil {
			clog.Fatalf("Failed to start admin panel: %v", err)
		}
	}()

	// --- Proxy (Ports 80 & 443) ---
	proxyHandler := proxy.NewProxy(store, stats, ipReputation, notifier)
	proxyMux := http.NewServeMux()
	proxyMux.Handle("/", proxyHandler)

	// Autocert for automatic HTTPS certificates
	certManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: store.HostPolicy, // Use the rule store to validate hosts
		Cache:      autocert.DirCache("certs"),
	}

	// HTTPS server
	server := &http.Server{
		Addr:    ":443",
		Handler: proxyMux,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	// HTTP server (for ACME challenge and redirecting to HTTPS)
	go func() {
		clog.Infof("Starting HTTP server on :80")
		if err := http.ListenAndServe(":80", certManager.HTTPHandler(nil)); err != nil {
			clog.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start HTTPS server
	clog.Infof("Starting HTTPS server on :443")
	if err := server.ListenAndServeTLS("", ""); err != nil {
		clog.Fatalf("HTTPS server error: %v", err)
	}
}
