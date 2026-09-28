package local

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"blog/internal/media/app"
)

var ErrTooLarge = errors.New("upload exceeds size limit")
var ErrUnsupported = errors.New("unsupported media format")
var filePattern = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png|gif|webp|mp4|webm)$`)
var formats = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp", "video/mp4": ".mp4", "video/webm": ".webm"}

type Storage struct{ dir string }

func New(dir string) (*Storage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Storage{dir: dir}, nil
}

func (s *Storage) Save(reader io.Reader, originalName string, limit int64) (app.Asset, error) {
	header := make([]byte, 512)
	n, err := io.ReadFull(reader, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return app.Asset{}, err
	}
	header = header[:n]
	mimeType := http.DetectContentType(header)
	ext, ok := formats[mimeType]
	if !ok {
		return app.Asset{}, ErrUnsupported
	}
	tmp, err := os.CreateTemp(s.dir, ".upload-*.tmp")
	if err != nil {
		return app.Asset{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if int64(n) > limit {
		return app.Asset{}, ErrTooLarge
	}
	if _, err := tmp.Write(header); err != nil {
		return app.Asset{}, err
	}
	written, err := io.Copy(tmp, io.LimitReader(reader, limit-int64(n)+1))
	if err != nil {
		return app.Asset{}, err
	}
	size := written + int64(n)
	if size > limit {
		return app.Asset{}, ErrTooLarge
	}
	if strings.HasPrefix(mimeType, "image/") && mimeType != "image/webp" {
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return app.Asset{}, err
		}
		config, _, err := image.DecodeConfig(tmp)
		if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 80_000_000 {
			return app.Asset{}, ErrUnsupported
		}
	}
	if err := tmp.Chmod(0o644); err != nil {
		return app.Asset{}, err
	}
	if err := tmp.Sync(); err != nil {
		return app.Asset{}, err
	}
	if err := tmp.Close(); err != nil {
		return app.Asset{}, err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return app.Asset{}, err
	}
	filename := hex.EncodeToString(random) + ext
	if err := os.Rename(tmp.Name(), filepath.Join(s.dir, filename)); err != nil {
		return app.Asset{}, err
	}
	kind := "image"
	if strings.HasPrefix(mimeType, "video/") {
		kind = "video"
	}
	return app.Asset{URL: "/media/" + filename, Name: filepath.Base(originalName), Type: kind, Size: size}, nil
}

func (s *Storage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !filePattern.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.dir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", name))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	// ServeContent supports byte ranges, needed for video seeking.
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func (s *Storage) Remove(url string) error {
	name := strings.TrimPrefix(url, "/media/")
	if !filePattern.MatchString(name) {
		return ErrUnsupported
	}
	return os.Remove(filepath.Join(s.dir, name))
}
