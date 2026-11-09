// Package marker manages the .mtm-data repo marker: a small YAML file at
// the data repo root recording what kind of repo it is. `mtm init` writes
// kind: real; the demo seeder writes kind: demo into sandbox repos and
// refuses to touch a repo whose marker says real.
package marker

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// Filename is the marker's name at the repo root.
	Filename = ".mtm-data"
	KindReal = "real"
	KindDemo = "demo"
)

// ErrNotExist reports that no marker was found.
var ErrNotExist = errors.New("no .mtm-data marker")

// Marker is the parsed .mtm-data file.
type Marker struct {
	ID      string `yaml:"id"`
	Created string `yaml:"created"` // local-naive date, 2006-01-02
	Kind    string `yaml:"kind"`    // real | demo
}

// Read parses the marker at root. ErrNotExist when absent.
func Read(root string) (Marker, error) {
	var m Marker
	data, err := os.ReadFile(filepath.Join(root, Filename))
	if os.IsNotExist(err) {
		return m, ErrNotExist
	}
	if err != nil {
		return m, err
	}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parse %s: %w", filepath.Join(root, Filename), err)
	}
	if m.ID == "" || (m.Kind != KindReal && m.Kind != KindDemo) {
		return m, fmt.Errorf("invalid %s: want a UUID id and kind real|demo", filepath.Join(root, Filename))
	}
	return m, nil
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// Write creates a fresh marker with kind and a new UUID at root.
// Overwrites an existing marker — callers that must not downgrade a
// real marker use Ensure instead.
func Write(root, kind string) (Marker, error) {
	id, err := newUUID()
	if err != nil {
		return Marker{}, err
	}
	m := Marker{ID: id, Created: time.Now().Format("2006-01-02"), Kind: kind}
	return m, writeFile(root, m)
}

// Ensure returns the existing marker, or writes a fresh one with kind
// when missing. An existing marker is never rewritten.
func Ensure(root, kind string) (Marker, bool, error) {
	m, err := Read(root)
	if err == nil {
		return m, false, nil
	}
	if !errors.Is(err, ErrNotExist) {
		return Marker{}, false, err
	}
	m, werr := Write(root, kind)
	return m, true, werr
}

func writeFile(root string, m Marker) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	path := filepath.Join(root, Filename)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FindUp walks up from dir looking for a .mtm-data marker. Returns the
// directory holding it and the parsed marker; ErrNotExist when no
// ancestor has one.
func FindUp(dir string) (string, Marker, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", Marker{}, err
	}
	for {
		if m, rerr := Read(abs); rerr == nil {
			return abs, m, nil
		} else if !errors.Is(rerr, ErrNotExist) {
			return abs, m, rerr
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", Marker{}, ErrNotExist
		}
		abs = parent
	}
}

// RootForTarget decides where the marker for a command writing into
// target belongs: an ancestor holding a marker (walk-up), else the
// parent of a directory named "projects" (the data-repo layout), else
// target itself.
func RootForTarget(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if root, _, ferr := FindUp(abs); ferr == nil {
		return root, nil
	} else if !errors.Is(ferr, ErrNotExist) {
		return "", ferr
	}
	if filepath.Base(abs) == "projects" {
		return filepath.Dir(abs), nil
	}
	return abs, nil
}
