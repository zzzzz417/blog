package posthttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"blog/internal/platform/httpx"
	"blog/internal/post/app"
	"blog/internal/post/domain"
)

type articleResponse struct {
	Slug        string      `json:"slug"`
	Title       string      `json:"title"`
	Summary     string      `json:"summary"`
	Category    string      `json:"category"`
	Kind        domain.Kind `json:"kind"`
	CoverImage  string      `json:"coverImage"`
	VideoURL    string      `json:"videoUrl"`
	Markdown    string      `json:"markdown,omitempty"`
	PublishedAt string      `json:"publishedAt"`
	Tags        []string    `json:"tags"`
}

type listResponse struct {
	Items []articleResponse `json:"items"`
	Total int               `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}

func Register(mux *http.ServeMux, service *app.Service, logger *slog.Logger, require func(http.Handler) http.Handler) {
	h := &handler{service: service, logger: logger}
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/posts", h.list)
	mux.HandleFunc("GET /api/v1/posts/{slug}", h.detail)
	mux.HandleFunc("GET /api/v1/tags", h.tags)
	if require != nil {
		mux.Handle("POST /api/v1/posts", require(http.HandlerFunc(h.create)))
		mux.Handle("PUT /api/v1/posts/{slug}", require(http.HandlerFunc(h.update)))
		mux.Handle("DELETE /api/v1/posts/{slug}", require(http.HandlerFunc(h.remove)))
	}
}

type handler struct {
	service *app.Service
	logger  *slog.Logger
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := optionalInt(r.URL.Query().Get("page"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "page must be an integer")
		return
	}
	limit, err := optionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}
	result, err := h.service.List(r.Context(), domain.ListQuery{
		Search: r.URL.Query().Get("q"),
		Sort:   domain.Sort(r.URL.Query().Get("sort")),
		Page:   page,
		Limit:  limit,
		Tag:    r.URL.Query().Get("tag"),
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	items := make([]articleResponse, 0, len(result.Items))
	for _, post := range result.Items {
		items = append(items, toResponse(post, false))
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Total: result.Total, Page: result.Page, Limit: result.Limit})
}

func (h *handler) detail(w http.ResponseWriter, r *http.Request) {
	post, err := h.service.BySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(post, true))
}

func (h *handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPost):
		writeError(w, 400, strings.TrimPrefix(err.Error(), domain.ErrInvalidPost.Error()+": "))
	case errors.Is(err, domain.ErrConflict):
		writeError(w, 409, "文章标识已存在，请换一个标识")
	case errors.Is(err, domain.ErrReadOnly):
		writeError(w, 503, "编辑功能尚未配置")
	case errors.Is(err, domain.ErrInvalidQuery):
		writeError(w, http.StatusBadRequest, "invalid query")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "article not found")
	default:
		h.logger.Error("request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func optionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func toResponse(post domain.Post, withMarkdown bool) articleResponse {
	response := articleResponse{
		Slug: post.Slug, Title: post.Title, Summary: post.Summary,
		Category: post.Category, Kind: post.Kind, CoverImage: post.CoverImage,
		VideoURL: post.VideoURL, PublishedAt: post.PublishedAt.Format(time.RFC3339),
		Tags: post.Tags,
	}
	if withMarkdown {
		response.Markdown = post.Markdown
	}
	return response
}

func (h *handler) tags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.service.Tags(r.Context())
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, tags)
}

type articleInput struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Category    string   `json:"category"`
	CoverImage  string   `json:"coverImage"`
	Markdown    string   `json:"markdown"`
	PublishedAt string   `json:"publishedAt"`
	Tags        []string `json:"tags"`
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) { h.save(w, r, true) }
func (h *handler) update(w http.ResponseWriter, r *http.Request) { h.save(w, r, false) }

func (h *handler) save(w http.ResponseWriter, r *http.Request, create bool) {
	var input articleInput
	if !httpx.Decode(w, r, 2<<20, &input) {
		return
	}
	if !create && r.PathValue("slug") != input.Slug {
		writeError(w, 400, "发布后不能更改文章标识")
		return
	}
	date, err := time.Parse("2006-01-02", input.PublishedAt)
	if err != nil {
		writeError(w, 400, "发布日期格式须为 YYYY-MM-DD")
		return
	}
	post, err := h.service.Save(r.Context(), domain.Post{Slug: input.Slug, Title: input.Title, Summary: input.Summary, Category: input.Category, CoverImage: input.CoverImage, Markdown: input.Markdown, PublishedAt: date, Tags: input.Tags}, create)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	status := http.StatusOK
	if create {
		status = http.StatusCreated
		w.Header().Set("Location", "/api/v1/posts/"+post.Slug)
	}
	writeJSON(w, status, toResponse(post, true))
}

func (h *handler) remove(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), r.PathValue("slug")); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func RequestLogger(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("request panic", "panic", recovered, "path", r.URL.Path)
				writeError(wrapped, http.StatusInternalServerError, "internal server error")
			}
			logger.Info("http request", "method", r.Method, "path", r.URL.Path,
				"status", wrapped.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(wrapped, r)
	})
}
