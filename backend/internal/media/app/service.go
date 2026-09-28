package app

import (
	"io"
)

type Asset struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type Storage interface {
	Save(io.Reader, string, int64) (Asset, error)
	Remove(string) error
}

type Service struct {
	storage  Storage
	maxBytes int64
}

func NewService(storage Storage, maxBytes int64) *Service {
	return &Service{storage: storage, maxBytes: maxBytes}
}
func (s *Service) Upload(reader io.Reader, name string) (Asset, error) {
	return s.storage.Save(reader, name, s.maxBytes)
}
func (s *Service) MaxBytes() int64          { return s.maxBytes }
func (s *Service) Discard(url string) error { return s.storage.Remove(url) }
