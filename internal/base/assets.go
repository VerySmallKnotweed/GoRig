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
)

//go:embed web/index.html web/app.css web/app.js web/vendor/bootstrap.css web/vendor/bootstrap.js web/vendor/LICENSE.bootstrap
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
	if r.Method == http.MethodGet {
		assets := map[string]struct{ path, contentType string }{
			"/assets/bootstrap.css": {"web/vendor/bootstrap.css", "text/css; charset=utf-8"},
			"/assets/bootstrap.js":  {"web/vendor/bootstrap.js", "text/javascript; charset=utf-8"},
			"/assets/app.css":       {"web/app.css", "text/css; charset=utf-8"},
			"/assets/app.js":        {"web/app.js", "text/javascript; charset=utf-8"},
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
				Notes  string  `json:"notes"`
				Status *string `json:"status"`
			}
			if !readJSON(w, r, &req) {
				return
			}
			respond(w, map[string]bool{"ok": true}, s.store.SaveDetails(id, req.Notes, req.Status))
			return
		}
	}
	if (r.URL.Path == "/api/preview" || r.URL.Path == "/api/import") && r.Method == http.MethodPost {
		var req struct {
			AssetID int64           `json:"asset_id"`
			Report  json.RawMessage `json:"report"`
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
		p, id, err := s.store.Import(req.Report, req.AssetID)
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
