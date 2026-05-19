package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

type FeedState struct {
	Seen []string `json:"seen"`
}

type State struct {
	mu    sync.Mutex
	path  string
	feeds map[string]*FeedState
}

func LoadState(path string) (*State, error) {
	s := &State{path: path, feeds: make(map[string]*FeedState)}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s.feeds); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	for url, fs := range s.feeds {
		if fs == nil {
			s.feeds[url] = &FeedState{}
		}
	}
	return s, nil
}

func (s *State) Has(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.feeds[url]
	return ok
}

func (s *State) SeenSet(url string) map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs := s.feeds[url]
	if fs == nil {
		return map[string]bool{}
	}
	set := make(map[string]bool, len(fs.Seen))
	for _, id := range fs.Seen {
		set[id] = true
	}
	return set
}

func (s *State) Append(url string, ids []string, maxSeen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs := s.feeds[url]
	if fs == nil {
		fs = &FeedState{}
		s.feeds[url] = fs
	}
	fs.Seen = append(fs.Seen, ids...)
	if maxSeen > 0 && len(fs.Seen) > maxSeen {
		fs.Seen = fs.Seen[len(fs.Seen)-maxSeen:]
	}
}

func (s *State) Replace(url string, ids []string, maxSeen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs := s.feeds[url]
	if fs == nil {
		fs = &FeedState{}
		s.feeds[url] = fs
	}
	if maxSeen > 0 && len(ids) > maxSeen {
		ids = ids[len(ids)-maxSeen:]
	}
	fs.Seen = append(fs.Seen[:0], ids...)
}

func (s *State) Save() error {
	s.mu.Lock()
	data, err := json.MarshalIndent(s.feeds, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp state: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}
