package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"open-physical-ai-dojo/backend/internal/domain"
)

type JSONLStore struct {
	mu              sync.Mutex
	dataDir         string
	episodesFile    string
	evaluationsFile string
}

func NewJSONLStore(dataDir string) (*JSONLStore, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	return &JSONLStore{
		dataDir:         dataDir,
		episodesFile:    filepath.Join(dataDir, "episodes.jsonl"),
		evaluationsFile: filepath.Join(dataDir, "evaluations.jsonl"),
	}, nil
}

func (s *JSONLStore) SaveEpisode(task *domain.Task) error {
	if task == nil {
		return errors.New("cannot save nil episode")
	}
	return s.appendJSON(s.episodesFile, task)
}

func (s *JSONLStore) ListEpisodes(limit int) ([]domain.Task, error) {
	var episodes []domain.Task
	if err := s.readJSONL(s.episodesFile, func(line []byte) error {
		var task domain.Task
		if err := json.Unmarshal(line, &task); err != nil {
			return err
		}
		episodes = append(episodes, task)
		return nil
	}); err != nil {
		return nil, err
	}
	slices.Reverse(episodes)
	return limitSlice(episodes, limit), nil
}

func (s *JSONLStore) SaveEvaluation(result domain.EvaluationResult) error {
	return s.appendJSON(s.evaluationsFile, result)
}

func (s *JSONLStore) ListEvaluations(limit int) ([]domain.EvaluationResult, error) {
	var evaluations []domain.EvaluationResult
	if err := s.readJSONL(s.evaluationsFile, func(line []byte) error {
		var result domain.EvaluationResult
		if err := json.Unmarshal(line, &result); err != nil {
			return err
		}
		evaluations = append(evaluations, result)
		return nil
	}); err != nil {
		return nil, err
	}
	slices.Reverse(evaluations)
	return limitSlice(evaluations, limit), nil
}

func (s *JSONLStore) appendJSON(path string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (s *JSONLStore) readJSONL(path string, decode func(line []byte) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := decode(scanner.Bytes()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func limitSlice[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
