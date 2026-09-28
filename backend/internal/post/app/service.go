package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"blog/internal/post/domain"
)

type Service struct {
	repo    domain.Repository
	source  domain.ContentStore
	maxTags int
	mu      sync.Mutex
}

func NewService(repo domain.Repository) *Service { return &Service{repo: repo, maxTags: 10} }

func (s *Service) WithEditing(source domain.ContentStore, maxTags int) *Service {
	s.source, s.maxTags = source, maxTags
	return s
}

func (s *Service) Tags(ctx context.Context) ([]domain.Tag, error) { return s.repo.Tags(ctx) }

func (s *Service) Save(ctx context.Context, post domain.Post, create bool) (domain.Post, error) {
	if err := post.Normalize(s.maxTags); err != nil {
		return domain.Post{}, err
	}
	err := s.mutate(ctx, post.Slug, create, &post)
	return post, err
}

func (s *Service) Delete(ctx context.Context, slug string) error {
	if !domain.ValidSlug(slug) {
		return domain.ErrInvalidQuery
	}
	return s.mutate(ctx, slug, false, nil)
}

func (s *Service) mutate(ctx context.Context, slug string, create bool, next *domain.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.source == nil {
		return domain.ErrReadOnly
	}
	posts, err := s.source.Load()
	if err != nil {
		return err
	}
	index := -1
	for i := range posts {
		if posts[i].Slug == slug {
			index = i
			break
		}
	}
	if create && index >= 0 {
		return domain.ErrConflict
	}
	if !create && index < 0 {
		return domain.ErrNotFound
	}
	var undo func() error
	if next == nil {
		undo, err = s.source.Remove(slug)
		posts = append(posts[:index], posts[index+1:]...)
	} else {
		undo, err = s.source.Write(*next)
		if index < 0 {
			posts = append(posts, *next)
		} else {
			posts[index] = *next
		}
	}
	if err != nil {
		return err
	}
	if err := s.repo.UpsertMany(ctx, posts); err != nil {
		if rollback := undo(); rollback != nil {
			return errors.Join(err, fmt.Errorf("restore Markdown source: %w", rollback))
		}
		return err
	}
	return nil
}

func (s *Service) Sync(ctx context.Context, posts []domain.Post) error {
	return s.repo.UpsertMany(ctx, posts)
}

func (s *Service) List(ctx context.Context, q domain.ListQuery) (domain.Page, error) {
	q.Search = strings.TrimSpace(q.Search)
	q.Tag = strings.TrimSpace(q.Tag)
	if utf8.RuneCountInString(q.Tag) > 24 {
		return domain.Page{}, domain.ErrInvalidQuery
	}
	if utf8.RuneCountInString(q.Search) > 120 || q.Page < 0 || q.Limit < 0 || q.Page > 10000 {
		return domain.Page{}, domain.ErrInvalidQuery
	}
	if q.Sort == "" {
		q.Sort = domain.SortNewest
	}
	if q.Sort != domain.SortNewest && q.Sort != domain.SortOldest {
		return domain.Page{}, domain.ErrInvalidQuery
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Limit == 0 {
		q.Limit = 10
	}
	if q.Limit > 50 {
		return domain.Page{}, domain.ErrInvalidQuery
	}
	return s.repo.List(ctx, q)
}

func (s *Service) BySlug(ctx context.Context, slug string) (domain.Post, error) {
	if slug == "" {
		return domain.Post{}, domain.ErrInvalidQuery
	}
	return s.repo.BySlug(ctx, slug)
}
