package proxy

import "net/http"

// Explicit allowlist: no directory browsing or arbitrary panel files on public sites.
var maintenanceAssets = map[string]string{
	"maintenance.css": "maintenance.css", "maintenance.js": "maintenance.js",
	"game_manager.js": "vendor/2048/game_manager.js", "grid.js": "vendor/2048/grid.js", "tile.js": "vendor/2048/tile.js",
	"game.js": "maintenance-game.js", "license.txt": "vendor/2048/LICENSE.txt",
	"network.svg": "icons/network.svg", "rotate-ccw.svg": "icons/rotate-ccw.svg",
	"qoncept.svg":  "icons/qoncept.svg",
	"arrow-up.svg": "icons/arrow-up.svg", "arrow-down.svg": "icons/arrow-down.svg",
	"arrow-left.svg": "icons/arrow-left.svg", "arrow-right.svg": "icons/arrow-right.svg",
	"sun-moon.svg": "icons/sun-moon.svg", "play.svg": "icons/play.svg",
}

func ServeMaintenanceAsset(w http.ResponseWriter, r *http.Request) bool {
	const prefix = "/_router/maintenance/assets/"
	if len(r.URL.Path) <= len(prefix) || r.URL.Path[:len(prefix)] != prefix {
		return false
	}
	file, ok := maintenanceAssets[r.URL.Path[len(prefix):]]
	if !ok {
		http.NotFound(w, r)
		return true
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeFile(w, r, "internal/panel/static/"+file)
	return true
}
