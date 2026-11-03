package base

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"GoRig/internal/icons"
)

//go:embed web/index.html web/app.css web/app.js web/vendor/bootstrap.css web/vendor/bootstrap.js web/vendor/LICENSE.bootstrap web/vendor/popper.min.js web/vendor/LICENSE.popper web/vendor/LICENSE.roboto web/vendor/fonts/*.woff2
var files embed.FS

const maxBody = 8 << 20

type server struct {
	store    *Store
	shutdown func()
}

// Handler exposes the local application; it is not intended for public use.
func Handler(store *Store) http.Handler { return &server{store: store} }

// HandlerWithShutdown requests shutdown asynchronously after the response.
func HandlerWithShutdown(store *Store, shutdown func()) http.Handler {
	return &server{store: store, shutdown: shutdown}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Prevent hostile websites from using the local service via DNS rebinding or CSRF.
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if host != "127.0.0.1" && host != "localhost" {
		http.Error(w, "только локальный доступ", http.StatusForbidden)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "http" || u.Host != r.Host {
			http.Error(w, "недопустимый источник запроса", http.StatusForbidden)
			return
		}
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; object-src 'none'; frame-ancestors 'none'")
	if r.URL.Path == "/" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data, _ := files.ReadFile("web/index.html")
		_, _ = w.Write(data)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/favicon.ico" {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(icons.BaseICO)
		return
	}
	if r.Method == http.MethodGet {
		assets := map[string]struct{ path, contentType string }{
			"/assets/bootstrap.css":                          {"web/vendor/bootstrap.css", "text/css; charset=utf-8"},
			"/assets/bootstrap.js":                           {"web/vendor/bootstrap.js", "text/javascript; charset=utf-8"},
			"/assets/popper.min.js":                          {"web/vendor/popper.min.js", "text/javascript; charset=utf-8"},
			"/assets/app.css":                                {"web/app.css", "text/css; charset=utf-8"},
			"/assets/app.js":                                 {"web/app.js", "text/javascript; charset=utf-8"},
			"/assets/fonts/roboto-latin-400-normal.woff2":    {"web/vendor/fonts/roboto-latin-400-normal.woff2", "font/woff2"},
			"/assets/fonts/roboto-cyrillic-400-normal.woff2": {"web/vendor/fonts/roboto-cyrillic-400-normal.woff2", "font/woff2"},
			"/assets/fonts/roboto-latin-500-normal.woff2":    {"web/vendor/fonts/roboto-latin-500-normal.woff2", "font/woff2"},
			"/assets/fonts/roboto-cyrillic-500-normal.woff2": {"web/vendor/fonts/roboto-cyrillic-500-normal.woff2", "font/woff2"},
			"/assets/fonts/roboto-latin-700-normal.woff2":    {"web/vendor/fonts/roboto-latin-700-normal.woff2", "font/woff2"},
			"/assets/fonts/roboto-cyrillic-700-normal.woff2": {"web/vendor/fonts/roboto-cyrillic-700-normal.woff2", "font/woff2"},
		}
		if asset, ok := assets[r.URL.Path]; ok {
			data, err := files.ReadFile(asset.path)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", asset.contentType)
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(data)
			return
		}
	}
	if r.URL.Path == "/api/shutdown" && r.Method == http.MethodPost {
		if s.shutdown == nil {
			http.Error(w, "выключение недоступно", http.StatusServiceUnavailable)
			return
		}
		var req struct{}
		if !readJSON(w, r, &req) {
			return
		}
		respond(w, map[string]bool{"ok": true}, nil)
		defer s.shutdown()
		return
	}
	if r.URL.Path == "/api/organizations" {
		switch r.Method {
		case http.MethodGet:
			out, err := s.store.Organizations()
			respond(w, out, err)
			return
		case http.MethodPost:
			var req struct {
				Name string `json:"name"`
			}
			if !readJSON(w, r, &req) {
				return
			}
			id, err := s.store.CreateOrganization(req.Name)
			respond(w, map[string]int64{"id": id}, err)
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/organizations/") {
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/organizations/"), 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "неверный ID", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			var req struct {
				Name string `json:"name"`
			}
			if !readJSON(w, r, &req) {
				return
			}
			respond(w, map[string]bool{"ok": true}, s.store.RenameOrganization(id, req.Name))
			return
		case http.MethodDelete:
			var req struct{}
			if !readJSON(w, r, &req) {
				return
			}
			respond(w, map[string]bool{"ok": true}, s.store.DeleteOrganization(id))
			return
		}
	}
	if r.URL.Path == "/api/print-report" && r.Method == http.MethodGet {
		scope := int64(-1)
		if raw := r.URL.Query().Get("organization_id"); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id < 0 {
				http.Error(w, "неверный ID организации", http.StatusBadRequest)
				return
			}
			scope = id
		}
		out, err := s.store.PrintableSnapshot(scope)
		respond(w, out, err)
		return
	}
	if r.URL.Path == "/api/assets" && r.Method == http.MethodGet {
		assets, err := s.store.Assets()
		respond(w, assets, err)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/assets/") {
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/assets/"), 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "неверный ID", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			history, err := s.store.History(id)
			respond(w, history, err)
			return
		case http.MethodDelete:
			// Require a JSON request body as an extra CSRF safeguard.
			var req struct{}
			if !readJSON(w, r, &req) {
				return
			}
			respond(w, map[string]bool{"ok": true}, s.store.Delete(id))
			return
		case http.MethodPut:
			var req struct {
				Notes          string  `json:"notes"`
				Status         *string `json:"status"`
				OrganizationID *int64  `json:"organization_id"`
			}
			if !readJSON(w, r, &req) {
				return
			}
			respond(w, map[string]bool{"ok": true}, s.store.SaveAssetDetails(id, AssetDetails{Notes: req.Notes, Status: req.Status, OrganizationID: req.OrganizationID}))
			return
		}
	}
	if (r.URL.Path == "/api/preview" || r.URL.Path == "/api/import") && r.Method == http.MethodPost {
		var req struct {
			AssetID int64           `json:"asset_id"`
			Report  json.RawMessage `json:"report"`
			Details *AssetDetails   `json:"details"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if req.AssetID < 0 {
			http.Error(w, "неверный ID", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/api/preview" {
			p, err := s.store.Preview(req.Report, req.AssetID)
			respond(w, p, err)
			return
		}
		p, id, err := s.store.ImportWithDetails(req.Report, req.AssetID, req.Details)
		respond(w, struct {
			Preview Preview `json:"preview"`
			AssetID int64   `json:"asset_id"`
		}{p, id}, err)
		return
	}
	http.NotFound(w, r)
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "ожидается application/json", http.StatusUnsupportedMediaType)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "не удалось прочитать запрос: "+err.Error(), http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "лишние данные в запросе", http.StatusBadRequest)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprint(err), status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}
