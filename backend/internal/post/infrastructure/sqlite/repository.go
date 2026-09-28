package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"

	"blog/internal/post/domain"
)

type Repository struct {
	db     *sql.DB
	hasFTS bool
}

func NewRepository(db *sql.DB, hasFTS bool) *Repository {
	return &Repository{db: db, hasFTS: hasFTS}
}

func (r *Repository) UpsertMany(ctx context.Context, posts []domain.Post) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin content sync: %w", err)
	}
	defer tx.Rollback()
	// Markdown files are the source of truth. Use a temporary table so the
	// deletion step is safe even when the blog grows beyond SQLite's bind limit.
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE IF NOT EXISTS sync_slugs (slug TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("prepare content sync: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sync_slugs`); err != nil {
		return fmt.Errorf("reset content sync: %w", err)
	}
	const statement = `INSERT INTO posts
 (slug, title, summary, category, kind, cover_image, video_url, body, published_at, updated_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
 ON CONFLICT(slug) DO UPDATE SET
 title=excluded.title, summary=excluded.summary, category=excluded.category,
 kind=excluded.kind, cover_image=excluded.cover_image, video_url=excluded.video_url,
 body=excluded.body, published_at=excluded.published_at, updated_at=excluded.updated_at`
	now := time.Now().Unix()
	for _, post := range posts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO sync_slugs (slug) VALUES (?)`, post.Slug); err != nil {
			return fmt.Errorf("track article %q: %w", post.Slug, err)
		}
		if _, err := tx.ExecContext(ctx, statement, post.Slug, post.Title, post.Summary,
			post.Category, post.Kind, post.CoverImage, post.VideoURL, post.Markdown,
			post.PublishedAt.Unix(), now); err != nil {
			return fmt.Errorf("save article %q: %w", post.Slug, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM post_tags WHERE post_id = (SELECT id FROM posts WHERE slug = ?)`, post.Slug); err != nil {
			return err
		}
		for _, tag := range post.Tags {
			if _, err := tx.ExecContext(ctx, `INSERT INTO tags(name) VALUES (?) ON CONFLICT(name) DO NOTHING`, tag); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO post_tags(post_id, tag_id) SELECT p.id, t.id FROM posts p, tags t WHERE p.slug = ? AND t.name = ?`, post.Slug, tag); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM posts WHERE slug NOT IN (SELECT slug FROM sync_slugs)`); err != nil {
		return fmt.Errorf("remove deleted articles: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit content sync: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, q domain.ListQuery) (domain.Page, error) {
	from := "FROM posts p"
	where := "WHERE 1=1"
	var args []any
	if q.Tag != "" {
		where += ` AND EXISTS (SELECT 1 FROM post_tags pt JOIN tags t ON t.id = pt.tag_id WHERE pt.post_id = p.id AND t.name = ?)`
		args = append(args, q.Tag)
	}
	if q.Search != "" {
		if r.hasFTS && isLatinText(q.Search) {
			from += " JOIN posts_fts ON posts_fts.rowid = p.id"
			where += " AND posts_fts MATCH ?"
			args = append(args, `"`+q.Search+`"`)
		} else {
			where += " AND (instr(lower(p.title), lower(?)) > 0 OR instr(lower(p.summary), lower(?)) > 0 OR instr(lower(p.body), lower(?)) > 0)"
			args = append(args, q.Search, q.Search, q.Search)
		}
	}
	result := domain.Page{Items: []domain.Post{}, Page: q.Page, Limit: q.Limit}
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) "+from+" "+where, args...).Scan(&result.Total); err != nil {
		return domain.Page{}, fmt.Errorf("count articles: %w", err)
	}
	order := "DESC"
	if q.Sort == domain.SortOldest {
		order = "ASC"
	}
	listArgs := append(append([]any(nil), args...), q.Limit, (q.Page-1)*q.Limit)
	query := `SELECT p.slug, p.title, p.summary, p.category, p.kind, p.cover_image,
 p.video_url, p.published_at ` + from + " " + where +
		" ORDER BY p.published_at " + order + ", p.id " + order + " LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return domain.Page{}, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var post domain.Post
		var published int64
		if err := rows.Scan(&post.Slug, &post.Title, &post.Summary, &post.Category,
			&post.Kind, &post.CoverImage, &post.VideoURL, &published); err != nil {
			return domain.Page{}, fmt.Errorf("read article: %w", err)
		}
		post.PublishedAt = time.Unix(published, 0).UTC()
		result.Items = append(result.Items, post)
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, fmt.Errorf("iterate articles: %w", err)
	}
	if err := rows.Close(); err != nil {
		return domain.Page{}, err
	}
	for i := range result.Items {
		result.Items[i].Tags, err = r.postTags(ctx, result.Items[i].Slug)
		if err != nil {
			return domain.Page{}, err
		}
	}
	return result, nil
}

func (r *Repository) BySlug(ctx context.Context, slug string) (domain.Post, error) {
	var post domain.Post
	var published int64
	err := r.db.QueryRowContext(ctx, `SELECT slug, title, summary, category, kind,
 cover_image, video_url, body, published_at FROM posts WHERE slug = ?`, slug).
		Scan(&post.Slug, &post.Title, &post.Summary, &post.Category, &post.Kind,
			&post.CoverImage, &post.VideoURL, &post.Markdown, &published)
	if err == sql.ErrNoRows {
		return domain.Post{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Post{}, fmt.Errorf("find article: %w", err)
	}
	post.PublishedAt = time.Unix(published, 0).UTC()
	post.Tags, err = r.postTags(ctx, slug)
	if err != nil {
		return domain.Post{}, err
	}
	return post, nil
}

func (r *Repository) postTags(ctx context.Context, slug string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.name FROM tags t JOIN post_tags pt ON pt.tag_id = t.id JOIN posts p ON p.id = pt.post_id WHERE p.slug = ? ORDER BY t.id`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

func (r *Repository) Tags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.name, COUNT(pt.post_id) FROM tags t LEFT JOIN post_tags pt ON pt.tag_id = t.id GROUP BY t.id ORDER BY COUNT(pt.post_id) DESC, t.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []domain.Tag{}
	for rows.Next() {
		var tag domain.Tag
		if err := rows.Scan(&tag.Name, &tag.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func isLatinText(q string) bool {
	for _, char := range q {
		if char > unicode.MaxASCII || !(unicode.IsLetter(char) || unicode.IsDigit(char) || char == ' ') {
			return false
		}
	}
	return strings.TrimSpace(q) != ""
}
