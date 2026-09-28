package server

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"blog/internal/config"
	editorapp "blog/internal/editor/app"
	editorhttp "blog/internal/editor/transport/http"
	mediaapp "blog/internal/media/app"
	"blog/internal/media/infrastructure/local"
	mediahttp "blog/internal/media/transport/http"
	"blog/internal/platform/httpx"
	postapp "blog/internal/post/app"
	posthttp "blog/internal/post/transport/http"
)

func NewRouter(posts *postapp.Service, auth *editorapp.Service, storage *local.Storage, cfg config.Config, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	editor := editorhttp.New(auth, cfg.CookieSecure, cfg.MaxTags, cfg.UploadMaxBytes)
	editor.Register(mux)
	posthttp.Register(mux, posts, logger, editor.Require)
	mux.Handle("POST /api/v1/media", editor.Require(mediahttp.Upload(mediaapp.NewService(storage, cfg.UploadMaxBytes), logger)))
	mux.Handle("GET /media/{name}", storage)
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) { httpx.Error(w, 404, "接口不存在") })
	if info, err := os.Stat(filepath.Join(cfg.WebDir, "index.html")); err == nil && !info.IsDir() {
		files := http.FileServer(http.Dir(cfg.WebDir))
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(cfg.WebDir, filepath.Clean("/"+r.URL.Path))
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
			if strings.Contains(filepath.Base(r.URL.Path), ".") {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, filepath.Join(cfg.WebDir, "index.html"))
		})
	}
	return posthttp.RequestLogger(httpx.SameOrigin(mux), logger)
}
