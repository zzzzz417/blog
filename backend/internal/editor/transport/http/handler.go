package editorhttp

import (
	"net"
	"net/http"
	"sync"
	"time"

	"blog/internal/editor/app"
	"blog/internal/platform/httpx"
)

const cookieName = "listening_editor"

type attempts struct {
	count int
	until time.Time
}

type Handler struct {
	auth      *app.Service
	secure    bool
	maxTags   int
	maxUpload int64
	mu        sync.Mutex
	attempts  map[string]attempts
}

func New(auth *app.Service, secure bool, maxTags int, maxUpload int64) *Handler {
	return &Handler{auth: auth, secure: secure, maxTags: maxTags, maxUpload: maxUpload, attempts: map[string]attempts{}}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/session", h.session)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	expires, ok := h.authenticated(r)
	result := map[string]any{"authenticated": ok, "enabled": h.auth.Enabled(), "maxTags": h.maxTags, "maxUploadBytes": h.maxUpload}
	if ok {
		result["expiresAt"] = expires.UTC().Format(time.RFC3339)
	}
	httpx.JSON(w, 200, result)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.auth.Enabled() {
		httpx.Error(w, 503, "编辑功能尚未配置，请设置后端编辑密钥")
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !h.allowAttempt(ip) {
		w.Header().Set("Retry-After", "900")
		httpx.Error(w, 429, "尝试次数过多，请 15 分钟后再试")
		return
	}
	var input struct {
		Key string `json:"key"`
	}
	if !httpx.Decode(w, r, 4096, &input) {
		return
	}
	token, expires, err := h.auth.Login(input.Key)
	if err != nil {
		httpx.Error(w, 401, "密钥不正确")
		return
	}
	h.mu.Lock()
	delete(h.attempts, ip)
	h.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/api", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: max(1, int(time.Until(expires).Seconds()))})
	httpx.JSON(w, 200, map[string]any{"authenticated": true, "expiresAt": expires.UTC().Format(time.RFC3339)})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/api", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) authenticated(r *http.Request) (time.Time, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return time.Time{}, false
	}
	expires, err := h.auth.Verify(cookie.Value)
	return expires, err == nil
}

func (h *Handler) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.authenticated(r); !ok {
			httpx.Error(w, 401, "请先登录编辑者账号")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) allowAttempt(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for key, value := range h.attempts {
		if now.After(value.until) {
			delete(h.attempts, key)
		}
	}
	entry, exists := h.attempts[ip]
	if !exists {
		if len(h.attempts) >= 4096 {
			return false
		}
		entry.until = now.Add(15 * time.Minute)
	}
	if entry.count >= 5 {
		return false
	}
	entry.count++
	h.attempts[ip] = entry
	return true
}
