package server_test

import (
	"blog/internal/config"
	editorapp "blog/internal/editor/app"
	"blog/internal/media/infrastructure/local"
	"blog/internal/platform/database"
	postapp "blog/internal/post/app"
	"blog/internal/post/domain"
	"blog/internal/post/infrastructure/markdown"
	postsqlite "blog/internal/post/infrastructure/sqlite"
	"blog/internal/server"
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const editorKey = "local-integration-test-editor-key"

type fixture struct {
	handler http.Handler
	posts   *postapp.Service
	source  *markdown.Store
	dir     string
	close   func() error
}

func setup(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	db, fts, err := database.Open(filepath.Join(dir, "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	source := markdown.NewStore(filepath.Join(dir, "posts"))
	if _, err := source.Load(); err != nil {
		t.Fatal(err)
	}
	posts := postapp.NewService(postsqlite.NewRepository(db, fts)).WithEditing(source, 10)
	storage, err := local.New(filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{WebDir: filepath.Join(dir, "web"), UploadMaxBytes: 1 << 20, MaxTags: 10}
	if err := os.MkdirAll(cfg.WebDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.WebDir, "index.html"), []byte("<!doctype html><title>Listening</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	auth := editorapp.NewService(editorKey, strings.Repeat("test-secret", 4), time.Hour)
	handler := server.NewRouter(posts, auth, storage, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return fixture{handler, posts, source, dir, db.Close}
}

func call(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie, status int) *httptest.ResponseRecorder {
	t.Helper()
	var data io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, data)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	result := httptest.NewRecorder()
	h.ServeHTTP(result, req)
	if result.Code != status {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, result.Code, status, result.Body)
	}
	return result
}

func login(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	result := call(t, h, "POST", "/api/v1/auth/login", map[string]string{"key": editorKey}, nil, 200)
	cookies := result.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("missing protected JWT cookie")
	}
	if strings.Contains(result.Body.String(), cookies[0].Value) {
		t.Fatal("JWT exposed in response body")
	}
	return cookies[0]
}

func article(slug string) map[string]any {
	return map[string]any{
		"slug": slug, "title": "混合内容", "summary": "图文与视频", "category": "测试", "coverImage": "",
		"publishedAt": "2026-09-28", "tags": []string{"旅行", "Go"},
		"markdown": "第一段 searchable-body\n\n![山](/images/mountain-rain.jpg)\n\n中间文字\n\n<video controls src=\"/videos/seaside-dusk.mp4\"></video>\n\n![海](/images/seaside-dusk.jpg)\n\n末尾文字",
	}
}

func TestEditorCRUDTagsAndPersistence(t *testing.T) {
	f := setup(t)
	call(t, f.handler, "GET", "/api/v1/posts", nil, nil, 200)
	call(t, f.handler, "POST", "/api/v1/posts", article("private"), nil, 401)
	call(t, f.handler, "PUT", "/api/v1/posts/private", article("private"), nil, 401)
	call(t, f.handler, "DELETE", "/api/v1/posts/private", nil, nil, 401)
	call(t, f.handler, "POST", "/api/v1/auth/login", map[string]string{"key": "wrong"}, nil, 401)
	cookie := login(t, f.handler)
	session := call(t, f.handler, "GET", "/api/v1/auth/session", nil, cookie, 200)
	if !strings.Contains(session.Body.String(), "\"authenticated\":true") || strings.Contains(session.Body.String(), editorKey) {
		t.Fatal("invalid session response")
	}
	input := article("mixed-post")
	call(t, f.handler, "POST", "/api/v1/posts", input, cookie, 201)
	call(t, f.handler, "POST", "/api/v1/posts", input, cookie, 409)
	raw, err := os.ReadFile(filepath.Join(f.dir, "posts", "mixed-post.md"))
	if err != nil || !strings.Contains(string(raw), input["markdown"].(string)) {
		t.Fatalf("Markdown not persisted: %v", err)
	}
	reloaded, err := markdown.LoadDir(filepath.Join(f.dir, "posts"))
	if err != nil || len(reloaded) != 1 || len(reloaded[0].Tags) != 2 || reloaded[0].CoverImage != "" {
		t.Fatalf("restart source invalid: %v %#v", err, reloaded)
	}
	if err := f.posts.Sync(t.Context(), reloaded); err != nil {
		t.Fatal(err)
	}
	call(t, f.handler, "GET", "/api/v1/posts/mixed-post", nil, nil, 200)
	filtered := call(t, f.handler, "GET", "/api/v1/posts?tag=Go&q=searchable-body&sort=oldest", nil, nil, 200)
	if !strings.Contains(filtered.Body.String(), "\"total\":1") {
		t.Fatal("combined tag/body/sort query failed")
	}
	empty := call(t, f.handler, "GET", "/api/v1/posts?tag=missing&q=searchable-body", nil, nil, 200)
	if !strings.Contains(empty.Body.String(), "\"total\":0") {
		t.Fatal("tag filter ignored")
	}
	input["tags"] = []string{"旅行", "Go", "go"}
	input["title"] = "更新后的混合内容"
	call(t, f.handler, "PUT", "/api/v1/posts/mixed-post", input, cookie, 200)
	tags := call(t, f.handler, "GET", "/api/v1/tags", nil, nil, 200)
	var catalog []domain.Tag
	if err := json.Unmarshal(tags.Body.Bytes(), &catalog); err != nil || len(catalog) != 2 || catalog[0].Count != 1 {
		t.Fatalf("tags not deduplicated: %s", tags.Body)
	}
	input["tags"] = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}
	call(t, f.handler, "PUT", "/api/v1/posts/mixed-post", input, cookie, 400)
	input["tags"] = []string{"生活"}
	input["coverImage"] = "/images/../../secret.png"
	call(t, f.handler, "PUT", "/api/v1/posts/mixed-post", input, cookie, 400)
	input["coverImage"] = ""
	input["slug"] = "../escape"
	call(t, f.handler, "POST", "/api/v1/posts", input, cookie, 400)
	req := httptest.NewRequest("DELETE", "/api/v1/posts/mixed-post", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://foreign.example")
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, req)
	if recorder.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
	call(t, f.handler, "DELETE", "/api/v1/posts/mixed-post", nil, cookie, 204)
	call(t, f.handler, "GET", "/api/v1/posts/mixed-post", nil, nil, 404)
	if _, err := os.Stat(filepath.Join(f.dir, "posts", "mixed-post.md")); !os.IsNotExist(err) {
		t.Fatal("deleted Markdown still exists")
	}
	tags = call(t, f.handler, "GET", "/api/v1/tags", nil, nil, 200)
	if strings.Contains(tags.Body.String(), "\"count\":1") {
		t.Fatal("deleted article kept a tag reference")
	}
	loggedOut := call(t, f.handler, "POST", "/api/v1/auth/logout", nil, cookie, 204)
	if loggedOut.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not expire cookie")
	}
}

func multipartRequest(t *testing.T, data []byte, filename string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	req := httptest.NewRequest("POST", "/api/v1/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestMediaAuthorizationTypeLimitsAndRanges(t *testing.T) {
	f := setup(t)
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	req := multipartRequest(t, pngData.Bytes(), "../../escape.png")
	unauthorized := httptest.NewRecorder()
	f.handler.ServeHTTP(unauthorized, req)
	if unauthorized.Code != 401 {
		t.Fatal("unauthorized upload accepted")
	}
	cookie := login(t, f.handler)
	for _, test := range []struct {
		name string
		data []byte
		want int
	}{
		{"../../escape.png", pngData.Bytes(), 201},
		{"pretend.jpg", []byte("<script>alert(1)</script>"), 415},
		{"vector.svg", []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"), 415},
		{"too-large.png", append(append([]byte(nil), pngData.Bytes()...), make([]byte, 1<<20)...), 413},
		{"video.mp4", append([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}, make([]byte, 1024)...), 201},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := multipartRequest(t, test.data, test.name)
			req.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			f.handler.ServeHTTP(recorder, req)
			if recorder.Code != test.want {
				t.Fatalf("upload got %d want %d: %s", recorder.Code, test.want, recorder.Body)
			}
			if test.want != 201 {
				return
			}
			var asset struct{ URL string }
			json.Unmarshal(recorder.Body.Bytes(), &asset)
			if !strings.HasPrefix(asset.URL, "/media/") || strings.Contains(asset.URL, "escape") {
				t.Fatal("unsafe filename")
			}
			get := httptest.NewRequest("GET", asset.URL, nil)
			get.Header.Set("Range", "bytes=0-7")
			out := httptest.NewRecorder()
			f.handler.ServeHTTP(out, get)
			if out.Code != 206 || out.Body.Len() != 8 || out.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("media range failed: %d %s", out.Code, out.Body)
			}
		})
	}
	files, _ := os.ReadDir(filepath.Join(f.dir, "uploads"))
	if len(files) != 2 {
		t.Fatalf("rejected uploads left files: %d", len(files))
	}
}

func TestSourceRollbackOnIndexFailure(t *testing.T) {
	f := setup(t)
	created, err := f.posts.Save(t.Context(), domain.Post{Slug: "rollback", Title: "原文", Markdown: "正文", PublishedAt: time.Now(), Tags: []string{"Go"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(f.dir, "posts", "rollback.md"))
	f.close()
	created.Title = "不应保留的更新"
	if _, err := f.posts.Save(t.Context(), created, false); err == nil {
		t.Fatal("expected SQLite failure")
	}
	after, _ := os.ReadFile(filepath.Join(f.dir, "posts", "rollback.md"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed update changed source")
	}
	if err := f.posts.Delete(t.Context(), "rollback"); err == nil {
		t.Fatal("expected SQLite delete failure")
	}
	after, _ = os.ReadFile(filepath.Join(f.dir, "posts", "rollback.md"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed delete was not rolled back")
	}
	created.Slug = "not-created"
	if _, err := f.posts.Save(t.Context(), created, true); err == nil {
		t.Fatal("expected insert failure")
	}
	if _, err := os.Stat(filepath.Join(f.dir, "posts", "not-created.md")); !os.IsNotExist(err) {
		t.Fatal("failed creation left a file")
	}
}

func TestLoginThrottled(t *testing.T) {
	f := setup(t)
	for i := 0; i < 5; i++ {
		call(t, f.handler, "POST", "/api/v1/auth/login", map[string]string{"key": "wrong"}, nil, 401)
	}
	call(t, f.handler, "POST", "/api/v1/auth/login", map[string]string{"key": editorKey}, nil, 429)
}
