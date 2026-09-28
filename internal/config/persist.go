package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var saveMu sync.Mutex

// Save writes cfg back to path while preserving the existing file layout.
//
// The on-disk file is merged rather than overwritten: comments and unknown keys
// survive, and a value written as an environment reference such as
// ${OPENAI_API_KEY} stays a reference when the runtime value still equals its
// expansion. Secret-like keys that are backed by an environment reference are
// never replaced with plaintext; such changes stay runtime-only.
func Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	saveMu.Lock()
	defer saveMu.Unlock()

	freshBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	var fresh yaml.Node
	if err := yaml.Unmarshal(freshBytes, &fresh); err != nil {
		return fmt.Errorf("reparse config: %w", err)
	}

	var doc yaml.Node
	rawBytes, err := os.ReadFile(path)
	switch {
	case err == nil && len(bytes.TrimSpace(rawBytes)) > 0:
		if err := yaml.Unmarshal(rawBytes, &doc); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
		mergeYAMLNode(docRoot(&doc), docRoot(&fresh), "")
	case err == nil || os.IsNotExist(err):
		doc = fresh
	default:
		return fmt.Errorf("read config file: %w", err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return writeFileAtomic(path, buf.Bytes(), 0o600)
}

func docRoot(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

func mergeYAMLNode(dst, src *yaml.Node, path string) {
	switch {
	case dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(src.Content); i += 2 {
			key, value := src.Content[i], src.Content[i+1]
			childPath := joinPath(path, key.Value)
			if existing := mappingValue(dst, key.Value); existing != nil {
				mergeYAMLNode(existing, value, childPath)
				continue
			}
			dst.Content = append(dst.Content, key, value)
		}
	case dst.Kind == yaml.SequenceNode && src.Kind == yaml.SequenceNode:
		for i, item := range src.Content {
			if i < len(dst.Content) {
				mergeYAMLNode(dst.Content[i], item, fmt.Sprintf("%s[%d]", path, i))
				continue
			}
			dst.Content = append(dst.Content, item)
		}
		if len(dst.Content) > len(src.Content) {
			dst.Content = dst.Content[:len(src.Content)]
		}
	case dst.Kind == yaml.ScalarNode && src.Kind == yaml.ScalarNode:
		mergeScalar(dst, src, path)
	default:
		*dst = *src
	}
}

func mergeScalar(dst, src *yaml.Node, path string) {
	if strings.Contains(dst.Value, "${") {
		if expandEnvWithDefault(dst.Value) == src.Value {
			return
		}
		if isSecretPath(path) {
			log.Printf("[Config] %s is backed by an environment reference; runtime change not persisted", path)
			return
		}
	}
	dst.Value = src.Value
	dst.Tag = src.Tag
	dst.Style = src.Style
}

func mappingValue(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

var secretPathMarkers = []string{"api_key", "apikey", "password", "secret", "token", "authorization", "dsn"}

func isSecretPath(path string) bool {
	lower := strings.ToLower(path)
	for _, marker := range secretPathMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}
