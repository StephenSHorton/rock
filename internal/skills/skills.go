// Package skills loads SKILL.md directories from the project and the user home.
package skills

import (
	"os"
	"path/filepath"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Body        string
	Path        string
}

func Discover(cwd string) ([]Skill, error) {
	home, _ := os.UserHomeDir()
	roots := []string{
		filepath.Join(cwd, ".rock", "skills"),
		filepath.Join(cwd, ".agents", "skills"),
		filepath.Join(cwd, ".cursor", "skills"),
		filepath.Join(cwd, ".claude", "skills"),
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".rock", "skills"))
	}
	var out []Skill
	seen := map[string]bool{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(root, e.Name(), "SKILL.md")
			if !e.IsDir() && strings.EqualFold(e.Name(), "SKILL.md") {
				path = filepath.Join(root, e.Name())
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			sk := parse(path, string(raw))
			if sk.Name == "" || seen[sk.Name] {
				continue
			}
			seen[sk.Name] = true
			out = append(out, sk)
		}
	}
	return out, nil
}

func parse(path, raw string) Skill {
	name := filepath.Base(filepath.Dir(path))
	desc := ""
	body := raw
	if strings.HasPrefix(raw, "---\n") || strings.HasPrefix(raw, "---\r\n") {
		rest := raw[4:]
		if i := strings.Index(rest, "\n---"); i >= 0 {
			front := rest[:i]
			body = strings.TrimSpace(rest[i+4:])
			for _, line := range strings.Split(front, "\n") {
				line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				v = strings.Trim(strings.TrimSpace(v), `"'`)
				switch strings.ToLower(strings.TrimSpace(k)) {
				case "name":
					name = v
				case "description":
					desc = v
				}
			}
		}
	}
	return Skill{Name: name, Description: desc, Body: strings.TrimSpace(body), Path: path}
}

func ByName(all []Skill, names []string) []Skill {
	if len(names) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []Skill
	for _, s := range all {
		if want[s.Name] {
			out = append(out, s)
		}
	}
	return out
}
