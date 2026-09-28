package database

import (
	"path/filepath"
	"testing"
)

func TestOpenRemovesOldFTSTriggersWithoutFTS5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blog.db")
	db, hasFTS, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if hasFTS {
		db.Close()
		t.Skip("requires a SQLite build without FTS5")
	}
	if _, err := db.Exec(`CREATE TRIGGER posts_ai AFTER INSERT ON posts BEGIN
		INSERT INTO posts_fts(rowid) VALUES (new.id);
	END;
	CREATE TRIGGER posts_ad AFTER DELETE ON posts BEGIN
		INSERT INTO posts_fts(rowid) VALUES (old.id);
	END;
	CREATE TRIGGER posts_au AFTER UPDATE ON posts BEGIN
		INSERT INTO posts_fts(rowid) VALUES (new.id);
	END;`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, hasFTS, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if hasFTS {
		t.Fatal("FTS5 unexpectedly available")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master
		WHERE type = 'trigger' AND name IN ('posts_ai', 'posts_ad', 'posts_au')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("old FTS5 triggers remain: %d", count)
	}
	if _, err := db.Exec(`INSERT INTO posts
		(slug, title, summary, category, kind, body, published_at, updated_at)
		VALUES ('test', 'Test', '', '', 'article', 'Body', 1, 1)`); err != nil {
		t.Fatalf("saving an article after reopening: %v", err)
	}
}
