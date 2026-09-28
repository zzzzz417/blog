package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS posts (
 id INTEGER PRIMARY KEY,
 slug TEXT NOT NULL UNIQUE,
 title TEXT NOT NULL,
 summary TEXT NOT NULL,
 category TEXT NOT NULL,
 kind TEXT NOT NULL CHECK (kind IN ('article', 'video')),
 cover_image TEXT NOT NULL DEFAULT '',
 video_url TEXT NOT NULL DEFAULT '',
 body TEXT NOT NULL,
 published_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS posts_published_at_idx ON posts(published_at DESC, id DESC);
CREATE TABLE IF NOT EXISTS tags (
 id INTEGER PRIMARY KEY,
 name TEXT NOT NULL UNIQUE COLLATE NOCASE
);
CREATE TABLE IF NOT EXISTS post_tags (
 post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
 PRIMARY KEY (post_id, tag_id)
);
CREATE INDEX IF NOT EXISTS post_tags_tag_idx ON post_tags(tag_id, post_id);
`

const ftsSchema = `
CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
 title, summary, body, content='posts', content_rowid='id', tokenize='unicode61'
);
CREATE TRIGGER posts_ai AFTER INSERT ON posts BEGIN
 INSERT INTO posts_fts(rowid, title, summary, body) VALUES (new.id, new.title, new.summary, new.body);
END;
CREATE TRIGGER posts_ad AFTER DELETE ON posts BEGIN
 INSERT INTO posts_fts(posts_fts, rowid, title, summary, body) VALUES ('delete', old.id, old.title, old.summary, old.body);
END;
CREATE TRIGGER posts_au AFTER UPDATE ON posts BEGIN
 INSERT INTO posts_fts(posts_fts, rowid, title, summary, body) VALUES ('delete', old.id, old.title, old.summary, old.body);
 INSERT INTO posts_fts(rowid, title, summary, body) VALUES (new.id, new.title, new.summary, new.body);
END;
INSERT INTO posts_fts(posts_fts) VALUES ('rebuild');
`

const dropFTSTriggers = `
DROP TRIGGER IF EXISTS posts_ai;
DROP TRIGGER IF EXISTS posts_ad;
DROP TRIGGER IF EXISTS posts_au;
`

// Open applies the schema and enables FTS5 when the SQLite build provides it.
// The repository still searches all article text when FTS5 is unavailable.
func Open(path string) (*sql.DB, bool, error) {
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, false, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, false, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{"PRAGMA busy_timeout = 5000", "PRAGMA journal_mode = WAL", "PRAGMA foreign_keys = ON", schema} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, false, fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	hasFTS, err := initializeSearch(db)
	if err != nil {
		db.Close()
		return nil, false, err
	}
	return db, hasFTS, nil
}

func initializeSearch(db *sql.DB) (bool, error) {
	hasFTS, err := supportsFTS5(db)
	if err != nil {
		return false, fmt.Errorf("check FTS5 support: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin search index setup: %w", err)
	}
	defer tx.Rollback()
	// A database created by an FTS5-enabled build still has these triggers when
	// reopened by a build without FTS5. Remove them before syncing posts.
	if _, err := tx.Exec(dropFTSTriggers); err != nil {
		return false, fmt.Errorf("reset search triggers: %w", err)
	}
	if hasFTS {
		// Rebuild after any period without FTS5, when writes did not update its index.
		if _, err := tx.Exec(ftsSchema); err != nil {
			return false, fmt.Errorf("initialize search index: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit search index setup: %w", err)
	}
	return hasFTS, nil
}

func supportsFTS5(db *sql.DB) (bool, error) {
	_, err := db.Exec("CREATE VIRTUAL TABLE temp.__blog_fts5_probe USING fts5(value)")
	if err != nil {
		if strings.Contains(err.Error(), "no such module: fts5") {
			return false, nil
		}
		return false, err
	}
	if _, err := db.Exec("DROP TABLE temp.__blog_fts5_probe"); err != nil {
		return false, err
	}
	return true, nil
}
