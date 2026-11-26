package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServiceRef struct {
	Name      string     `json:"name"`
	Additions []Addition `json:"additions,omitempty"`
}

type Stack struct {
	Name      string   `json:"name"`
	Templates []string `json:"templates"`
}

func (s *Store) stackDir(name string) string {
	return filepath.Join(s.root, "stacks", name)
}

func (s *Store) stackFile(name string) string {
	return filepath.Join(s.stackDir(name), "docker-compose.yml")
}

func (s *Store) CreateStack(stack Stack) (Entry, error) {
	name := strings.TrimSpace(stack.Name)
	if !namePattern.MatchString(name) {
		return Entry{}, fmt.Errorf("stack name %q is not valid", name)
	}
	if err := s.checkTemplates(stack.Templates); err != nil {
		return Entry{}, err
	}
	path := s.stackFile(name)
	if _, err := os.Stat(path); err == nil {
		return Entry{}, ErrExists
	}
	text, err := RenderStack(stack)
	if err != nil {
		return Entry{}, err
	}
	if err := os.MkdirAll(s.stackDir(name), 0o755); err != nil {
		return Entry{}, err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return Entry{}, err
	}
	return Entry{Name: name, Path: "stacks/" + name + "/docker-compose.yml"}, nil
}

func (s *Store) UpdateStack(name string, stack Stack) (Entry, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return Entry{}, fmt.Errorf("stack name %q is not valid", name)
	}
	if err := s.checkTemplates(stack.Templates); err != nil {
		return Entry{}, err
	}
	path := s.stackFile(name)
	if _, err := os.Stat(path); err != nil {
		return Entry{}, err
	}
	stack.Name = name
	text, err := RenderStack(stack)
	if err != nil {
		return Entry{}, err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return Entry{}, err
	}
	return Entry{Name: name, Path: "stacks/" + name + "/docker-compose.yml"}, nil
}

func (s *Store) Stack(name string) (Stack, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return Stack{}, fmt.Errorf("stack name %q is not valid", name)
	}
	content, err := os.ReadFile(s.stackFile(name))
	if err != nil {
		return Stack{}, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return Stack{}, err
	}
	stack := Stack{Name: name, Templates: []string{}}
	if include, ok := doc["include"].([]any); ok {
		for _, item := range include {
			var paths []string
			switch typed := item.(type) {
			case string:
				paths = append(paths, typed)
			case map[string]any:
				switch entry := typed["path"].(type) {
				case string:
					paths = append(paths, entry)
				case []any:
					for _, value := range entry {
						paths = append(paths, stringOf(value))
					}
				}
			}
			for _, item := range paths {
				path := strings.TrimPrefix(item, "../../")
				if strings.HasPrefix(path, "services/") {
					stack.Templates = append(stack.Templates, path)
				}
			}
		}
	}
	return stack, nil
}

func (s *Store) ReadStack(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("stack name %q is not valid", name)
	}
	content, err := os.ReadFile(s.stackFile(name))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *Store) DeleteStack(name string) error {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return fmt.Errorf("stack name %q is not valid", name)
	}
	if _, err := os.Stat(s.stackDir(name)); err != nil {
		return err
	}
	return os.RemoveAll(s.stackDir(name))
}

func (s *Store) Catalog() ([]ServiceRef, error) {
	entries, err := s.List()
	if err != nil {
		return nil, err
	}
	refs := make([]ServiceRef, 0, len(entries))
	for _, entry := range entries {
		spec, err := s.Spec(entry.Name)
		if err != nil {
			return nil, err
		}
		ref := ServiceRef{Name: entry.Name}
		for _, addition := range spec.Additions {
			ref.Additions = append(ref.Additions, addition)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func (s *Store) ListStacks() ([]Entry, error) {
	dir := filepath.Join(s.root, "stacks")
	items, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Entry{}, nil
		}
		return nil, err
	}
	entries := []Entry{}
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		if _, err := os.Stat(s.stackFile(item.Name())); err != nil {
			continue
		}
		entries = append(entries, Entry{Name: item.Name(), Path: "stacks/" + item.Name() + "/docker-compose.yml"})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func (s *Store) checkTemplates(templates []string) error {
	if len(templates) == 0 {
		return fmt.Errorf("a stack includes no template")
	}
	seen := map[string]bool{}
	for _, template := range templates {
		template = strings.TrimSpace(template)
		if !strings.HasPrefix(template, "services/") || strings.Contains(template, "..") || strings.HasPrefix(template, "/") {
			return fmt.Errorf("template %q must be a path under services/", template)
		}
		if seen[template] {
			return fmt.Errorf("template %q is included twice", template)
		}
		seen[template] = true
		if _, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(template))); err != nil {
			return fmt.Errorf("template %q is not stored", template)
		}
	}
	return nil
}

func RenderStack(stack Stack) (string, error) {
	order := []string{}
	groups := map[string][]string{}
	for _, template := range stack.Templates {
		template = strings.TrimSpace(template)
		if template == "" {
			continue
		}
		parts := strings.Split(template, "/")
		service := ""
		if len(parts) > 1 {
			service = parts[1]
		}
		if _, ok := groups[service]; !ok {
			order = append(order, service)
		}
		groups[service] = append(groups[service], template)
	}
	include := []any{}
	for _, service := range order {
		templates := groups[service]
		if len(templates) == 1 {
			include = append(include, map[string]any{"path": "../../" + templates[0]})
			continue
		}
		paths := make([]any, 0, len(templates))
		for _, template := range templates {
			paths = append(paths, "../../"+template)
		}
		include = append(include, map[string]any{"path": paths})
	}
	if len(include) == 0 {
		return "", fmt.Errorf("a stack includes no template")
	}
	doc := map[string]any{"include": include}
	var out strings.Builder
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	return out.String(), nil
}
