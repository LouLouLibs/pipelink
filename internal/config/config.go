package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// StringOrSlice handles TOML fields that can be a single string or an array of strings.
type StringOrSlice struct {
	values []string
}

// Strings returns the underlying string slice.
func (s StringOrSlice) Strings() []string {
	return s.values
}

// IsEmpty returns true if no values are set.
func (s StringOrSlice) IsEmpty() bool {
	return len(s.values) == 0
}

// UnmarshalTOML implements custom TOML unmarshaling for string-or-array fields.
func (s *StringOrSlice) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case string:
		s.values = []string{v}
	case []interface{}:
		s.values = make([]string, 0, len(v))
		for _, item := range v {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("expected string in array, got %T", item)
			}
			s.values = append(s.values, str)
		}
	default:
		return fmt.Errorf("expected string or array, got %T", data)
	}
	return nil
}

// Metadata describes the link entry type and purpose.
type Metadata struct {
	Type        string   `toml:"type"`
	Description string   `toml:"description"`
	GeneratedBy []string `toml:"generated_by"`
}

// Source specifies where to link from.
type Source struct {
	Directory string        `toml:"directory"`
	File      StringOrSlice `toml:"file"`
	Task      string        `toml:"task"`
}

// Target specifies where to link to.
type Target struct {
	Directory string        `toml:"directory"`
	File      StringOrSlice `toml:"file"`
}

// Entry represents one link specification in the config.
type Entry struct {
	Metadata Metadata `toml:"metadata"`
	Source   Source   `toml:"source"`
	Target   Target   `toml:"target"`
}

// LoadConfig reads a TOML file and returns the parsed entries.
func LoadConfig(path string) (map[string]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var entries map[string]Entry
	if err := toml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing TOML: %w", err)
	}

	// Default target.file to source.file when omitted
	for name, e := range entries {
		if e.Target.File.IsEmpty() && !e.Source.File.IsEmpty() {
			e.Target.File = e.Source.File
			entries[name] = e
		}
	}

	return entries, nil
}
