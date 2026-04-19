package api

import (
	"net"
	"net/http"
	"strings"
)

// HostGuard bloque les requêtes dont Host header n'est pas dans une
// allowlist connue. Parade au DNS rebinding : un site malveillant
// qui résout `evil.example` → 127.0.0.1 peut viser localhost:8080,
// mais le navigateur envoie Host:evil.example. Sans validation,
// notre API répond en 200 et le malicious JS drain les données.
//
// Allowlist par défaut couvre les modes d'accès légitimes au
// dashboard local :
//   - localhost / 127.0.0.1 / [::1] avec ou sans port
//   - les IPs privées RFC1918 (pour un accès LAN maison)
//   - les hosts *.local (mDNS Bonjour)
//
// Contournable via la variable d'env HIVE_EXTRA_HOSTS="host1,host2"
// pour un opérateur qui exposerait Hive derrière un reverse proxy.
func HostGuard(extraHosts []string, h http.Handler) http.Handler {
	extra := make(map[string]struct{}, len(extraHosts))
	for _, e := range extraHosts {
		e = strings.TrimSpace(strings.ToLower(e))
		if e != "" {
			extra[e] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		// Strip port pour comparaison.
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.ToLower(host)
		if isSafeHost(host, extra) {
			h.ServeHTTP(w, r)
			return
		}
		http.Error(w, "host not allowed", http.StatusForbidden)
	})
}

func isSafeHost(host string, extra map[string]struct{}) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]" {
		return true
	}
	if _, ok := extra[host]; ok {
		return true
	}
	// *.local (mDNS)
	if strings.HasSuffix(host, ".local") {
		return true
	}
	// IPs privées RFC1918 + link-local.
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}
