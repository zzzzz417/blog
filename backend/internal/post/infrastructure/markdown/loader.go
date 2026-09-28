package markdown

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"blog/internal/post/domain"
)

// LoadDir reads Markdown files with a deliberately small front matter format:
// one key: value pair per line, bounded by --- lines. It has no YAML features.
func LoadDir(dir string) ([]domain.Post, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("open content directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("content path %s is not a directory", dir)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("find article files: %w", err)
	}
	sort.Strings(files)
	posts := make([]domain.Post, 0, len(files))
	seen := make(map[string]bool)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		post, err := Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
		if seen[post.Slug] {
			return nil, fmt.Errorf("duplicate slug %q", post.Slug)
		}
		seen[post.Slug] = true
		posts = append(posts, post)
	}
	return posts, nil
}

func Parse(raw string) (domain.Post, error) {
	raw = strings.TrimPrefix(raw, "\ufeff")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) < 4 || strings.TrimSpace(lines[0]) != "---" {
		return domain.Post{}, fmt.Errorf("missing opening front matter marker")
	}
	meta := make(map[string]string)
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
		key, value, ok := strings.Cut(lines[i], ":")
		if !ok || strings.TrimSpace(key) == "" {
			return domain.Post{}, fmt.Errorf("invalid front matter line %d", i+1)
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, `"`) {
			decoded, err := strconv.Unquote(value)
			if err != nil {
				return domain.Post{}, fmt.Errorf("invalid quoted value on line %d", i+1)
			}
			value = decoded
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = strings.Trim(value, "'")
		}
		meta[strings.TrimSpace(key)] = value
	}
	if end < 0 {
		return domain.Post{}, fmt.Errorf("missing closing front matter marker")
	}
	for _, key := range []string{"slug", "title", "published_at"} {
		if meta[key] == "" {
			return domain.Post{}, fmt.Errorf("missing %s", key)
		}
	}
	if !domain.ValidSlug(meta["slug"]) {
		return domain.Post{}, fmt.Errorf("invalid slug %q", meta["slug"])
	}
	published, err := time.Parse("2006-01-02", meta["published_at"])
	if err != nil {
		return domain.Post{}, fmt.Errorf("published_at must use YYYY-MM-DD: %w", err)
	}
	kind := domain.Kind(meta["kind"])
	if kind == "" {
		kind = domain.KindArticle
	}
	if kind != domain.KindArticle && kind != domain.KindVideo {
		return domain.Post{}, fmt.Errorf("kind must be article or video")
	}
	cover := meta["cover_image"]
	video := meta["video_url"]
	if video != "" && !strings.HasPrefix(video, "/videos/") {
		return domain.Post{}, fmt.Errorf("video_url must start with /videos/")
	}
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	if body == "" {
		return domain.Post{}, fmt.Errorf("article body is empty")
	}
	// Old video front matter remains readable; new articles embed media in Markdown.
	if video != "" && !strings.Contains(body, video) {
		body = "<video controls playsinline preload=\"metadata\" src=\"" + html.EscapeString(video) + "\"></video>\n\n" + body
	}
	var tags []string
	if rawTags := strings.TrimSpace(meta["tags"]); rawTags != "" {
		if strings.HasPrefix(rawTags, "[") {
			if err := json.Unmarshal([]byte(rawTags), &tags); err != nil {
				return domain.Post{}, fmt.Errorf("tags must be an array of strings")
			}
		} else {
			tags = strings.Split(rawTags, ",")
		}
	}
	post := domain.Post{
		Slug:        meta["slug"],
		Title:       meta["title"],
		Summary:     meta["summary"],
		Category:    meta["category"],
		Kind:        kind,
		CoverImage:  cover,
		VideoURL:    video,
		Markdown:    body,
		PublishedAt: published,
		Tags:        tags,
	}
	if err := post.Normalize(0); err != nil {
		return domain.Post{}, err
	}
	return post, nil
}
