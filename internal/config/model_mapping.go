package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/flyingnobita/llml/internal/models"
)

// ModelEntryFromFile converts a discovered model to a cache entry.
func ModelEntryFromFile(f models.ModelFile) ModelEntry {
	return ModelEntry{
		Backend:    f.Backend.String(),
		ID:         f.Identity(),
		Path:       f.Path,
		Location:   f.DisplayLocation(),
		Name:       f.Name,
		Size:       f.Size,
		ModTime:    f.ModTime,
		Parameters: f.Parameters,
	}
}

// ToModelFile converts a cache entry to [models.ModelFile].
func (e ModelEntry) ToModelFile() (models.ModelFile, error) {
	be, err := models.ParseBackend(e.Backend)
	if err != nil {
		return models.ModelFile{}, err
	}
	path := ""
	if strings.TrimSpace(e.Path) != "" {
		path = filepath.Clean(e.Path)
	}
	id := strings.TrimSpace(e.ID)
	if id == "" && path != "" {
		id = path
	}
	if id == "" {
		return models.ModelFile{}, errors.New("empty model identity")
	}
	if be != models.BackendOllama && (path == "" || path == ".") {
		return models.ModelFile{}, errors.New("empty model path")
	}
	location := strings.TrimSpace(e.Location)
	if location == "" {
		if path != "" {
			location = path
		} else {
			location = id
		}
	}
	return models.ModelFile{
		Backend:    be,
		ID:         id,
		Path:       path,
		Location:   location,
		Name:       e.Name,
		Size:       e.Size,
		ModTime:    e.ModTime,
		Parameters: e.Parameters,
	}, nil
}

// ModelFilesFromEntries converts cache entries to model files, skipping invalid rows.
func ModelFilesFromEntries(entries []ModelEntry) []models.ModelFile {
	var out []models.ModelFile
	for _, e := range entries {
		f, err := e.ToModelFile()
		if err != nil {
			continue
		}
		out = append(out, f)
	}
	return out
}

// FilterExistingPaths keeps only models whose path still exists on disk.
func FilterExistingPaths(files []models.ModelFile) []models.ModelFile {
	var out []models.ModelFile
	for _, f := range files {
		if f.Backend == models.BackendOllama {
			out = append(out, f)
			continue
		}
		if _, err := os.Stat(f.Path); err != nil {
			continue
		}
		out = append(out, f)
	}
	return out
}
