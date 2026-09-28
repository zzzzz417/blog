package posthttp_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"blog/internal/platform/database"
	"blog/internal/post/app"
	"blog/internal/post/domain"
	postsqlite "blog/internal/post/infrastructure/sqlite"
	posthttp "blog/internal/post/transport/http"
)

func TestArticleAPI(t *testing.T) {
	db, hasFTS, err := database.Open(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := app.NewService(postsqlite.NewRepository(db, hasFTS))
	if err := service.Sync(t.Context(), []domain.Post{
		{Slug: "newer", Title: "山间来信", Summary: "一段旅行", Category: "旅行", Kind: domain.KindArticle, Markdown: "雨声和山风", PublishedAt: time.Date(2024, 6, 18, 0, 0, 0, 0, time.UTC)},
		{Slug: "older", Title: "旧相册", Summary: "照片", Category: "生活", Kind: domain.KindArticle, Markdown: "记得离线备份照片。lighthouse", PublishedAt: time.Date(2024, 4, 3, 0, 0, 0, 0, time.UTC)},
	}); err != nil {
		t.Fatal(err)
	}
	router := http.NewServeMux()
	posthttp.Register(router, service, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	get := func(target string, wantCode int) map[string]any {
		t.Helper()
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != wantCode {
			t.Fatalf("GET %s: status %d, body %s", target, recorder.Code, recorder.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	items := func(body map[string]any) []any { return body["items"].([]any) }
	slug := func(item any) string { return item.(map[string]any)["slug"].(string) }

	if got := slug(items(get("/api/v1/posts", http.StatusOK))[0]); got != "newer" {
		t.Fatalf("newest first: %s", got)
	}
	if got := slug(items(get("/api/v1/posts?sort=oldest", http.StatusOK))[0]); got != "older" {
		t.Fatalf("oldest first: %s", got)
	}
	for _, term := range []string{"离线备份", "lighthouse"} {
		body := get("/api/v1/posts?q="+url.QueryEscape(term), http.StatusOK)
		if body["total"] != float64(1) || slug(items(body)[0]) != "older" {
			t.Fatalf("body search %q: %#v", term, body)
		}
	}
	if _, found := get("/api/v1/posts", http.StatusOK)["items"].([]any)[0].(map[string]any)["markdown"]; found {
		t.Fatal("list response unexpectedly includes Markdown body")
	}
	if body := get("/api/v1/posts/older", http.StatusOK); body["markdown"] != "记得离线备份照片。lighthouse" {
		t.Fatalf("detail body: %#v", body)
	}
	get("/api/v1/posts/missing", http.StatusNotFound)
	get("/api/v1/posts?sort=sideways", http.StatusBadRequest)

	if err := service.Sync(t.Context(), []domain.Post{
		{Slug: "newer", Title: "山间来信", Summary: "一段旅行", Category: "旅行", Kind: domain.KindArticle, Markdown: "更新后的正文", PublishedAt: time.Date(2024, 6, 18, 0, 0, 0, 0, time.UTC)},
	}); err != nil {
		t.Fatal(err)
	}
	if body := get("/api/v1/posts", http.StatusOK); body["total"] != float64(1) {
		t.Fatalf("removed file remained in index: %#v", body)
	}
	get("/api/v1/posts/older", http.StatusNotFound)
}
