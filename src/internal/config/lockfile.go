package config

import (
	"fmt"
	"os"
	"strings"
)

// LockModule pins a resolved Goop module in goop.lock.
type LockModule struct {
	Path    string // canonical import path
	Version string // tag, commit, or version string
	Source  string // source location (often same as Path)
}

// Lockfile holds pinned module versions from goop.lock.
type Lockfile struct {
	Modules []LockModule
	byPath  map[string]LockModule
}

// LoadLockfile reads goop.lock from path. Missing file returns empty lockfile.
func LoadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Lockfile{byPath: make(map[string]LockModule)}, nil
		}
		return nil, fmt.Errorf("reading lockfile %s: %w", path, err)
	}
	return parseLockfile(string(data))
}

func parseLockfile(data string) (*Lockfile, error) {
	lf := &Lockfile{byPath: make(map[string]LockModule)}
	var cur *LockModule
	lines := strings.Split(data, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[[module]]" {
			if cur != nil && cur.Path != "" {
				lf.Modules = append(lf.Modules, *cur)
				lf.byPath[cur.Path] = *cur
			}
			cur = &LockModule{}
			continue
		}
		if cur == nil {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			val = strings.Trim(val, `"`)
			switch key {
			case "path":
				cur.Path = val
			case "version":
				cur.Version = val
			case "source":
				cur.Source = val
			}
		}
	}
	if cur != nil && cur.Path != "" {
		lf.Modules = append(lf.Modules, *cur)
		lf.byPath[cur.Path] = *cur
	}
	return lf, nil
}

// Lookup returns a pinned module by canonical path.
func (lf *Lockfile) Lookup(path string) (LockModule, bool) {
	if lf == nil {
		return LockModule{}, false
	}
	m, ok := lf.byPath[path]
	return m, ok
}

// Format writes the lockfile contents.
func (lf *Lockfile) Format() string {
	var buf strings.Builder
	for i, m := range lf.Modules {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("[[module]]\n")
		buf.WriteString(fmt.Sprintf("path = %q\n", m.Path))
		buf.WriteString(fmt.Sprintf("version = %q\n", m.Version))
		buf.WriteString(fmt.Sprintf("source = %q\n", m.Source))
	}
	return buf.String()
}

// Upsert adds or replaces a module pin.
func (lf *Lockfile) Upsert(m LockModule) {
	if lf.byPath == nil {
		lf.byPath = make(map[string]LockModule)
	}
	if _, ok := lf.byPath[m.Path]; !ok {
		lf.Modules = append(lf.Modules, m)
	} else {
		for i, existing := range lf.Modules {
			if existing.Path == m.Path {
				lf.Modules[i] = m
				break
			}
		}
	}
	lf.byPath[m.Path] = m
}
