package markdown

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"blog/internal/post/domain"
)

type Store struct{ dir string }

func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) Load() ([]domain.Post, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	return LoadDir(s.dir)
}

func (s *Store) find(slug string) (string, []byte, error) {
	if !domain.ValidSlug(slug) {
		return "", nil, domain.ErrInvalidQuery
	}
	files, err := filepath.Glob(filepath.Join(s.dir, "*.md"))
	if err != nil {
		return "", nil, err
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return "", nil, err
		}
		post, err := Parse(string(raw))
		if err != nil {
			return "", nil, err
		}
		if post.Slug == slug {
			return file, raw, nil
		}
	}
	return "", nil, domain.ErrNotFound
}

func (s *Store) Write(post domain.Post) (func() error, error) {
	if !domain.ValidSlug(post.Slug) {
		return nil, domain.ErrInvalidPost
	}
	file, previous, err := s.find(post.Slug)
	exists := err == nil
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if !exists {
		file = filepath.Join(s.dir, post.Slug+".md")
		if _, err := os.Lstat(file); err == nil {
			return nil, domain.ErrConflict
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if err := atomicWrite(file, serialize(post)); err != nil {
		return nil, err
	}
	return func() error {
		if exists {
			return atomicWrite(file, previous)
		}
		return os.Remove(file)
	}, nil
}

func (s *Store) Remove(slug string) (func() error, error) {
	file, raw, err := s.find(slug)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(file); err != nil {
		return nil, err
	}
	return func() error { return atomicWrite(file, raw) }, nil
}

func serialize(post domain.Post) []byte {
	tags, _ := json.Marshal(post.Tags)
	var out strings.Builder
	fmt.Fprintf(&out, "---\nslug: %s\ntitle: %s\nsummary: %s\ncategory: %s\npublished_at: %s\ntags: %s\n", post.Slug, strconv.Quote(post.Title), strconv.Quote(post.Summary), strconv.Quote(post.Category), post.PublishedAt.Format("2006-01-02"), tags)
	if post.CoverImage != "" {
		fmt.Fprintf(&out, "cover_image: %s\n", strconv.Quote(post.CoverImage))
	}
	fmt.Fprintf(&out, "---\n\n%s\n", post.Markdown)
	return []byte(out.String())
}

func atomicWrite(file string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), ".article-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := tmp.Write(content); err != nil {
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}
