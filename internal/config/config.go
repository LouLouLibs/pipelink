package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/louloulibs/pipelink/internal/remote"
)

// StringOrSlice handles TOML fields that can be a single string or an array of strings.
type StringOrSlice struct {
	values []string
}

// Strings builds a StringOrSlice from values.
func Strings(values ...string) StringOrSlice {
	return StringOrSlice{values: values}
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
	License     string   `toml:"license"` // informational, for remote entries
}

// Source specifies where to link from. Remote entries (type "remote") use
// URI, SHA256 and Cache instead of Directory and File.
type Source struct {
	Directory string        `toml:"directory"`
	File      StringOrSlice `toml:"file"`
	Task      string        `toml:"task"`
	URI       string        `toml:"uri"`
	SHA256    string        `toml:"sha256"`
	Cache     string        `toml:"cache"`
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

	// Default target.file to source.file (or, for remote entries, the
	// basename of source.uri) when omitted
	for name, e := range entries {
		if !e.Target.File.IsEmpty() {
			continue
		}
		if !e.Source.File.IsEmpty() {
			e.Target.File = e.Source.File
		} else if e.Metadata.Type == "remote" && e.Source.URI != "" {
			e.Target.File = StringOrSlice{values: []string{remote.BaseName(e.Source.URI)}}
		}
		entries[name] = e
	}

	return entries, nil
}
