package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedTemplates(t *testing.T, store *Store) {
	t.Helper()
	for _, image := range []string{"nginx:1.27", "postgres:16"} {
		if _, err := store.Create(Spec{Image: image}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStacksAreWrittenAndRead(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	seedTemplates(t, store)

	entry, err := store.CreateStack(Stack{Name: "site", Templates: []string{"services/nginx/docker-compose.yml", "services/postgres/docker-compose.yml"}})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Path != "stacks/site/docker-compose.yml" {
		t.Fatalf("path = %s", entry.Path)
	}
	content, err := os.ReadFile(filepath.Join(root, "stacks", "site", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"include:", "path: ../../services/nginx/docker-compose.yml", "path:", "../../services/postgres/docker-compose.yml",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("%q missing:\n%s", want, content)
		}
	}

	stack, err := store.Stack("site")
	if err != nil {
		t.Fatal(err)
	}
	if len(stack.Templates) != 2 || stack.Templates[0] != "services/nginx/docker-compose.yml" || stack.Templates[1] != "services/postgres/docker-compose.yml" {
		t.Fatalf("stack = %+v", stack)
	}

	entries, err := store.ListStacks()
	if err != nil || len(entries) != 1 || entries[0].Name != "site" {
		t.Fatalf("list = %v, %v", entries, err)
	}

	if _, err := store.UpdateStack("site", Stack{Name: "site", Templates: []string{"services/nginx/docker-compose.yml"}}); err != nil {
		t.Fatal(err)
	}
	stack, err = store.Stack("site")
	if err != nil || len(stack.Templates) != 1 {
		t.Fatalf("updated stack = %+v, %v", stack, err)
	}

	if err := store.DeleteStack("site"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stack("site"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read after delete = %v", err)
	}
}

func TestStacksRejectBadTemplates(t *testing.T) {
	store := NewStore(t.TempDir())
	seedTemplates(t, store)
	for _, bad := range []Stack{
		{Name: "site"},
		{Name: "site", Templates: []string{"services/nginx/docker-compose.yml", "services/nginx/docker-compose.yml"}},
		{Name: "site", Templates: []string{"services/ghost/docker-compose.yml"}},
		{Name: "bad name", Templates: []string{"services/nginx/docker-compose.yml"}},
	} {
		if _, err := store.CreateStack(bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	if _, err := store.CreateStack(Stack{Name: "site", Templates: []string{"services/nginx/docker-compose.yml"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateStack(Stack{Name: "site", Templates: []string{"services/nginx/docker-compose.yml"}}); !errors.Is(err, ErrExists) {
		t.Fatalf("second create = %v", err)
	}
}
