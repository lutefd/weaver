package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Export struct {
	Version     int         `json:"version"`
	ExportedAt  time.Time   `json:"exported_at"`
	Integration NamedRecipe `json:"integration"`
}

type NamedRecipe struct {
	Name string `json:"name"`
	Recipe
}

func NewExport(name string, recipe Recipe) (*Export, error) {
	if err := validateRecipe(name, recipe); err != nil {
		return nil, err
	}
	return &Export{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Integration: NamedRecipe{
			Name:   name,
			Recipe: normalizeRecipe(recipe),
		},
	}, nil
}

func EncodeExport(w io.Writer, state *Export) error {
	if state == nil {
		return fmt.Errorf("integration export is required")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(state)
}

func DecodeExport(r io.Reader) (*Export, error) {
	var state Export
	if err := json.NewDecoder(r).Decode(&state); err != nil {
		return nil, fmt.Errorf("decode integration export: %w", err)
	}
	if err := validateRecipe(state.Integration.Name, state.Integration.Recipe); err != nil {
		return nil, err
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return &state, nil
}
