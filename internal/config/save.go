// Package config — writing ~/.config/mtm/config.yaml.
// Load reads the file over the built-in defaults; Load never mutates it.
// SaveUIKeys is the one writer: it merges a set of ui.* keys into the
// existing document, leaving every other key, section and comment untouched.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// SaveUIKeys merges kv into the ui: mapping of the config file at path
// (empty = Path()). The file is created if it does not exist. The write
// is atomic (temp file in the same directory + rename) and the previous
// file is kept as <path>.bak. Unknown keys, other sections and comments
// are preserved. Values must be scalars (string, bool, int, float64).
func SaveUIKeys(path string, kv map[string]interface{}) error {
	if path == "" {
		path = Path()
	}
	doc := &yaml.Node{}
	mode := os.FileMode(0o644)
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if st, err := os.Stat(path); err == nil && st.Mode().Perm() != 0 {
			mode = st.Mode().Perm()
		}
		if err := yaml.Unmarshal(data, doc); err != nil {
			return fmt.Errorf("config %s: %w", path, err)
		}
		if doc.Kind == 0 { // empty file
			doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
		}
	case os.IsNotExist(err):
		doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	default:
		return fmt.Errorf("config %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config %s: top level must be a mapping", path)
	}
	uiNode := mappingValue(root, "ui")
	if uiNode == nil {
		uiNode = &yaml.Node{Kind: yaml.MappingNode}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "ui"}, uiNode)
	} else if uiNode.Kind != yaml.MappingNode {
		return fmt.Errorf("config %s: ui must be a mapping", path)
	}
	for _, k := range sortedKeys(kv) {
		setScalar(uiNode, k, kv[k])
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if data != nil {
		if err := os.WriteFile(path+".bak", data, mode); err != nil {
			return fmt.Errorf("config %s: backup: %w", path, err)
		}
	}
	tmp, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mappingValue finds key's value node in a mapping node.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func setScalar(mapping *yaml.Node, key string, value interface{}) {
	scalar := &yaml.Node{Kind: yaml.ScalarNode}
	switch v := value.(type) {
	case string:
		scalar.Value, scalar.Tag = v, "!!str"
	case bool:
		scalar.Value = fmt.Sprintf("%t", v)
		scalar.Tag = "!!bool"
	case int:
		scalar.Value = fmt.Sprintf("%d", v)
		scalar.Tag = "!!int"
	case float64:
		scalar.Value = fmt.Sprintf("%g", v)
		scalar.Tag = "!!float"
	case nil:
		scalar.Tag = "!!null"
	default:
		scalar.Value = fmt.Sprintf("%v", v)
		scalar.Tag = "!!str"
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = scalar
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key}, scalar)
}
