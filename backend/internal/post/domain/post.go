package domain

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrNotFound = errors.New("article not found")
var ErrInvalidQuery = errors.New("invalid query")
var ErrInvalidPost = errors.New("invalid article")
var ErrConflict = errors.New("article already exists")
var ErrReadOnly = errors.New("editing is not configured")

const MaxMarkdownBytes = 1 << 20

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Kind string

const (
	KindArticle Kind = "article"
	KindVideo   Kind = "video"
)

type Post struct {
	Slug        string
	Title       string
	Summary     string
	Category    string
	Kind        Kind
	CoverImage  string
	VideoURL    string
	Markdown    string
	PublishedAt time.Time
	Tags        []string
}

type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func ValidSlug(slug string) bool { return len(slug) <= 100 && slugPattern.MatchString(slug) }

func NormalizeTags(tags []string, limit int) ([]string, error) {
	result := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.Join(strings.Fields(tag), " ")
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > 24 {
			return nil, fmt.Errorf("%w: 每个标签最多 24 个字符", ErrInvalidPost)
		}
		key := strings.ToLower(tag)
		if !seen[key] {
			result = append(result, tag)
			seen[key] = true
		}
	}
	if limit > 0 && len(result) > limit {
		return nil, fmt.Errorf("%w: 每篇文章最多 %d 个标签", ErrInvalidPost, limit)
	}
	return result, nil
}

func (p *Post) Normalize(maxTags int) error {
	p.Title = strings.TrimSpace(p.Title)
	p.Summary = strings.TrimSpace(p.Summary)
	p.Category = strings.TrimSpace(p.Category)
	p.Markdown = strings.TrimSpace(p.Markdown)
	p.CoverImage = strings.TrimSpace(p.CoverImage)
	if !ValidSlug(p.Slug) {
		return fmt.Errorf("%w: 文章标识限 100 位小写字母、数字和连字符", ErrInvalidPost)
	}
	if p.Title == "" || utf8.RuneCountInString(p.Title) > 160 || strings.ContainsAny(p.Title, "\r\n") {
		return fmt.Errorf("%w: 标题需为 1–160 个字符的单行文字", ErrInvalidPost)
	}
	if p.Markdown == "" || len(p.Markdown) > MaxMarkdownBytes {
		return fmt.Errorf("%w: 正文不能为空，且不能超过 1 MB", ErrInvalidPost)
	}
	if p.Summary == "" {
		plain := []rune(strings.Join(strings.Fields(p.Markdown), " "))
		p.Summary = string(plain[:min(160, len(plain))])
	}
	if utf8.RuneCountInString(p.Summary) > 500 || strings.ContainsAny(p.Summary, "\r\n") {
		return fmt.Errorf("%w: 摘要限 500 个字符的单行文字", ErrInvalidPost)
	}
	if p.Category == "" {
		p.Category = "随记"
	}
	if utf8.RuneCountInString(p.Category) > 30 || strings.ContainsAny(p.Category, "\r\n") {
		return fmt.Errorf("%w: 分类限 30 个字符的单行文字", ErrInvalidPost)
	}
	if p.PublishedAt.IsZero() {
		return fmt.Errorf("%w: 请填写发布日期", ErrInvalidPost)
	}
	if p.CoverImage != "" {
		ext := strings.ToLower(path.Ext(p.CoverImage))
		if !(strings.HasPrefix(p.CoverImage, "/images/") || strings.HasPrefix(p.CoverImage, "/media/")) || path.Clean(p.CoverImage) != p.CoverImage || strings.ContainsAny(p.CoverImage, "\\\r\n?#") || !(ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp") {
			return fmt.Errorf("%w: 封面须为本站上传的图片", ErrInvalidPost)
		}
	}
	var err error
	p.Tags, err = NormalizeTags(p.Tags, maxTags)
	p.Kind, p.VideoURL = KindArticle, ""
	return err
}

type Sort string

const (
	SortNewest Sort = "newest"
	SortOldest Sort = "oldest"
)

type ListQuery struct {
	Search string
	Sort   Sort
	Page   int
	Limit  int
	Tag    string
}

type Page struct {
	Items []Post
	Total int
	Page  int
	Limit int
}

type Repository interface {
	UpsertMany(ctx context.Context, posts []Post) error
	List(ctx context.Context, query ListQuery) (Page, error)
	BySlug(ctx context.Context, slug string) (Post, error)
	Tags(ctx context.Context) ([]Tag, error)
}

// ContentStore keeps portable Markdown files as the durable source of truth.
// A mutation returns an undo operation so an index failure can be rolled back.
type ContentStore interface {
	Load() ([]Post, error)
	Write(Post) (undo func() error, err error)
	Remove(slug string) (undo func() error, err error)
}
