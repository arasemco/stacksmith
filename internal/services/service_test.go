package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderWritesEveryField(t *testing.T) {
	text, err := Render(Spec{
		Name:    "api",
		Image:   "node:20-alpine",
		Build:   &Build{Context: "./api"},
		Restart: "unless-stopped",
		Command: `["node", "server.js"]`,
		Ports:   []string{"3000:3000"},
		Environment: map[string]string{
			"NODE_ENV": "production",
		},
		Volumes: []string{"./data:/data"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"services:", "api:", "image: node:${NODE_TAG:?NODE_TAG is required}",
		"hostname: api", "domainname: ${STACK_DOMAIN:?STACK_DOMAIN is required}",
		"context: ./api", "restart: ${RESTART:-unless-stopped}", "- node", "- server.js",
		"- 3000:3000", "NODE_ENV: production", "- ./data:/data",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%q missing:\n%s", want, text)
		}
	}
}

func TestCreateStoresTheDocument(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	entry, err := store.Create(Spec{Image: "postgres:16"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Path != "services/postgres/docker-compose.yml" {
		t.Fatalf("path = %s", entry.Path)
	}
	data, err := os.ReadFile(filepath.Join(root, "services", "postgres", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "image: postgres:${POSTGRES_TAG:?POSTGRES_TAG is required}") {
		t.Fatalf("stored document:\n%s", data)
	}
	if _, err := store.Create(Spec{Image: "postgres:17"}); !errors.Is(err, ErrExists) {
		t.Fatalf("second create = %v, want ErrExists", err)
	}
}

func TestListAndRead(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, image := range []string{"nginx:1.27", "redis:7"} {
		if _, err := store.Create(Spec{Image: image}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "nginx" || entries[1].Name != "redis" {
		t.Fatalf("entries = %v", entries)
	}
	content, err := store.Read("redis")
	if err != nil || !strings.Contains(content, "redis:") {
		t.Fatalf("read = %q, %v", content, err)
	}
}

func TestNameComesFromTheImage(t *testing.T) {
	store := NewStore(t.TempDir())
	entry, err := store.Create(Spec{Image: "postgres:16"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Name != "postgres" {
		t.Fatalf("name = %s", entry.Name)
	}
	if _, err := store.Create(Spec{Image: ""}); err == nil {
		t.Fatal("a service without an image was accepted")
	}
}

func TestHostnameUsesTheStackDomain(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{Image: "nginx:1.27"}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "hostname: nginx") || !strings.Contains(content, "domainname: ${STACK_DOMAIN:?STACK_DOMAIN is required}") {
		t.Fatalf("hostname/domainname missing:\n%s", content)
	}
}

func TestImageTagIsARequiredVariable(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{Image: "nginx"}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "image: nginx:${NGINX_TAG:?NGINX_TAG is required}") {
		t.Fatalf("tag variable missing:\n%s", content)
	}
	if _, err := store.Create(Spec{Image: "ghcr.io/acme/app:1.2.3"}); err != nil {
		t.Fatal(err)
	}
	app, err := store.Read("app")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(app, "image: app:${APP_TAG:?APP_TAG is required}") {
		t.Fatalf("tag variable missing:\n%s", app)
	}
}

func TestNameMustMatchTheImage(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{Name: "db", Image: "postgres:16"}); err == nil {
		t.Fatal("a name that differs from the image was accepted")
	}
	if _, err := store.Create(Spec{Name: "postgres", Image: "postgres:16"}); err != nil {
		t.Fatal(err)
	}
}

func TestImageName(t *testing.T) {
	for image, want := range map[string]string{
		"postgres:16":            "postgres",
		"node:20-alpine":         "node",
		"ghcr.io/acme/app:1.2.3": "app",
		"registry:5000/app":      "app",
		"nginx":                  "nginx",
		"nginx@sha256:abc":       "nginx",
		"":                       "",
	} {
		if got := ImageName(image); got != want {
			t.Fatalf("ImageName(%q) = %q, want %q", image, got, want)
		}
	}
}

func TestBadNamesAreRejected(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, name := range []string{"two words", "../escape", "-leading"} {
		if _, err := store.Create(Spec{Name: name, Image: "nginx:1.27"}); err == nil {
			t.Fatalf("%q was accepted", name)
		}
	}
}

func TestUpdateRewritesTheDocument(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image:       "nginx:1.27",
		Ports:       []string{"80:80"},
		Environment: map[string]string{"A": "1"},
	}); err != nil {
		t.Fatal(err)
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Image != "nginx" || len(spec.Ports) != 1 || spec.Environment["A"] != "1" {

		t.Fatalf("spec = %+v", spec)
	}
	if _, err := store.Update("nginx", Spec{Image: "nginx:1.29", Ports: []string{"80:80", "443:443"}}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "443:443") {
		t.Fatalf("update not applied:\n%s", content)
	}
	if _, err := store.Update("nginx", Spec{Image: "redis:7"}); err == nil {
		t.Fatal("an update that changes the image name was accepted")
	}
}

func TestDeleteRemovesTheService(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{Image: "nginx:1.27"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("nginx"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read("nginx"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read after delete = %v", err)
	}
	if err := store.Delete("nginx"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second delete = %v", err)
	}
}

func TestHealthcheckIsRenderedAndRead(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image: "nginx:1.27",
		Healthcheck: &Healthcheck{
			Test:        "curl -f http://localhost/",
			Interval:    "10s",
			Timeout:     "5s",
			Retries:     3,
			StartPeriod: "20s",
		},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"healthcheck:", "test:", "- CMD-SHELL", "- curl -f http://localhost/",
		"interval: 10s", "timeout: 5s", "retries: 3", "start_period: 20s",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("%q missing:\n%s", want, content)
		}
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Healthcheck == nil || spec.Healthcheck.Test != "curl -f http://localhost/" || spec.Healthcheck.Retries != 3 {
		t.Fatalf("healthcheck = %+v", spec.Healthcheck)
	}
}

func TestRestartIsAnOptionalVariable(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{Image: "nginx:1.27", Restart: "unless-stopped"}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "restart: ${RESTART:-unless-stopped}") {
		t.Fatalf("restart variable missing:\n%s", content)
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Restart != "unless-stopped" {
		t.Fatalf("restart = %q", spec.Restart)
	}
	if _, err := store.Create(Spec{Image: "redis:7"}); err != nil {
		t.Fatal(err)
	}
	plain, err := store.Read("redis")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain, "restart: ${RESTART:-}") {
		t.Fatalf("restart default missing:\n%s", plain)
	}
}

func TestPullPolicyIsAnOptionalVariable(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{Image: "nginx:1.27", PullPolicy: "missing"}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "pull_policy: ${PULL_POLICY:-missing}") {
		t.Fatalf("pull policy variable missing:\n%s", content)
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if spec.PullPolicy != "missing" {
		t.Fatalf("pull policy = %q", spec.PullPolicy)
	}
	if _, err := store.Create(Spec{Image: "redis:7"}); err != nil {
		t.Fatal(err)
	}
	plain, err := store.Read("redis")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain, "pull_policy: ${PULL_POLICY:-}") {
		t.Fatalf("pull policy default missing:\n%s", plain)
	}
}

func TestMultilineEnvironmentIsDedented(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image: "wordpress:6",
		Environment: map[string]string{
			"WORDPRESS_CONFIG_EXTRA": "    line one\n        indented\n    line two\n",
		},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("wordpress")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "WORDPRESS_CONFIG_EXTRA: |") {
		t.Fatalf("block scalar missing:\n%s", content)
	}
	spec, err := store.Spec("wordpress")
	if err != nil {
		t.Fatal(err)
	}
	if got := spec.Environment["WORDPRESS_CONFIG_EXTRA"]; got != "line one\n    indented\nline two\n" {
		t.Fatalf("dedent = %q", got)
	}
}

func TestConfigsAreRenderedAndRead(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image: "nginx:1.27",
		Configs: []Config{{
			Source:  "site",
			Target:  "/etc/nginx/conf.d/site.conf",
			Content: "server {\n    listen 80;\n}",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"configs:", "- source: site", "target: /etc/nginx/conf.d/site.conf", "content: |",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("%q missing:\n%s", want, content)
		}
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Configs) != 1 || spec.Configs[0].Source != "site" || !strings.Contains(spec.Configs[0].Content, "listen 80") {
		t.Fatalf("configs = %+v", spec.Configs)
	}
	if err := Validate(Spec{Configs: []Config{{Source: "a"}, {Source: "a"}}}); err == nil {
		t.Fatal("duplicate config accepted")
	}
}

func TestConfigTypes(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image: "nginx:1.27",
		Configs: []Config{
			{Source: "file_conf", Type: "file", File: "./nginx.conf", Target: "/etc/nginx.conf"},
			{Source: "external_conf", Type: "external"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "file: ./nginx.conf") {
		t.Fatalf("file config missing:\n%s", content)
	}
	if strings.Contains(content, "  external_conf:") {
		t.Fatalf("external config declared at the top level:\n%s", content)
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Configs) != 2 || spec.Configs[0].Type != "file" || spec.Configs[1].Type != "external" {
		t.Fatalf("configs = %+v", spec.Configs)
	}
	for _, bad := range []Spec{
		{Configs: []Config{{Source: "a", Type: "file"}}},
		{Configs: []Config{{Source: "a", Type: "content"}}},
		{Configs: []Config{{Source: "a", Type: "nope", Content: "x"}}},
	} {
		if err := Validate(bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

func TestSecretsAreRenderedAndRead(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image: "nginx:1.27",
		Secrets: []Secret{
			{Source: "db_password", Type: "file", File: "./db_password", Target: "db_password"},
			{Source: "api_key", Type: "environment", Environment: "API_KEY"},
			{Source: "external_cert", Type: "external"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"secrets:", "file: ./db_password", "environment: API_KEY",
		"- source: db_password", "- source: api_key", "- source: external_cert",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("%q missing:\n%s", want, content)
		}
	}
	if strings.Contains(content, "  external_cert:") {
		t.Fatalf("external secret declared at the top level:\n%s", content)
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Secrets) != 3 || spec.Secrets[0].Type != "file" || spec.Secrets[1].Type != "environment" || spec.Secrets[2].Type != "external" {
		t.Fatalf("secrets = %+v", spec.Secrets)
	}
	for _, bad := range []Spec{
		{Secrets: []Secret{{Source: "a", Type: "file"}}},
		{Secrets: []Secret{{Source: "a", Type: "environment"}}},
		{Secrets: []Secret{{Source: "a", Type: "nope"}}},
		{Secrets: []Secret{{Source: "a", Type: "external"}, {Source: "a", Type: "external"}}},
	} {
		if err := Validate(bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

func TestDeployAndProfiles(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image:    "nginx:1.27",
		Profiles: []string{"frontend", "web"},
		Deploy:   &Deploy{CPUs: "1.5", Memory: "512M", Replicas: 2},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"profiles:", "- frontend", "- web", "deploy:", "resources:", "limits:",
		"cpus: \"1.5\"", "memory: 512M", "replicas: 2",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("%q missing:\n%s", want, content)
		}
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Profiles) != 2 || spec.Deploy == nil || spec.Deploy.CPUs != "1.5" || spec.Deploy.Replicas != 2 {
		t.Fatalf("spec = %+v", spec)
	}
	if _, err := store.Create(Spec{Image: "redis:7"}); err != nil {
		t.Fatal(err)
	}
	plain, err := store.Read("redis")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "deploy:") || strings.Contains(plain, "profiles:") {
		t.Fatalf("empty deploy/profiles written:\n%s", plain)
	}
}

func TestLabelsAreRenderedAndRead(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image:  "nginx:1.27",
		Labels: map[string]string{"traefik.enable": "true", "role": "edge"},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "labels:") || !strings.Contains(content, "traefik.enable: \"true\"") {
		t.Fatalf("labels missing:\n%s", content)
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Labels["role"] != "edge" {
		t.Fatalf("labels = %v", spec.Labels)
	}
	if _, err := store.Create(Spec{Image: "redis:7"}); err != nil {
		t.Fatal(err)
	}
	plain, err := store.Read("redis")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "labels:") {
		t.Fatalf("empty labels written:\n%s", plain)
	}
}

func TestMultilineLabelsAreBlocks(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image:  "nginx:1.27",
		Labels: map[string]string{"note": "    line one\n      line two"},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "note: |") {
		t.Fatalf("block label missing:\n%s", content)
	}
	spec, err := store.Spec("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if got := spec.Labels["note"]; got != "line one\n  line two\n" {
		t.Fatalf("label = %q", got)
	}
}

func TestAdditionsAreWrittenAndRead(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	disabled := false
	if _, err := store.Create(Spec{
		Image: "wordpress:6",
		Additions: []Addition{
			{Name: "mariadb", Environment: map[string]string{"WORDPRESS_DB_HOST": "mariadb"}},
			{Name: "redis", Depend: &disabled, Labels: map[string]string{"cache": "redis"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(filepath.Join(root, "services", "wordpress", "mariadb.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(database), "WORDPRESS_DB_HOST: mariadb") || !strings.Contains(string(database), "- mariadb") {
		t.Fatalf("mariadb addition:\n%s", database)
	}
	cache, err := os.ReadFile(filepath.Join(root, "services", "wordpress", "redis.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cache), "depends_on") {
		t.Fatalf("disabled depend still written:\n%s", cache)
	}
	spec, err := store.Spec("wordpress")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Additions) != 2 {
		t.Fatalf("additions = %+v", spec.Additions)
	}
	if _, err := store.Update("wordpress", Spec{
		Image:     "wordpress:6",
		Additions: []Addition{{Name: "mariadb"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "services", "wordpress", "redis.yml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale addition survived: %v", err)
	}
	off := false
	nonService := []Addition{{Name: "homepage/dashboard", Depend: &off, Labels: map[string]string{"homepage.name": "WordPress"}}}
	if err := Validate(Spec{Name: "wordpress", Additions: nonService}); err != nil {
		t.Fatalf("a non-service addition was rejected: %v", err)
	}
	pathAddition := []Addition{{Name: "database/mariadb"}}
	if err := Validate(Spec{Name: "wordpress", Additions: pathAddition}); err != nil {
		t.Fatalf("a path addition was rejected: %v", err)
	}
	for _, bad := range []Spec{
		{Name: "x", Additions: []Addition{{Name: "../escape"}}},
		{Name: "x", Additions: []Addition{{Name: "/abs"}}},
		{Name: "x", Additions: []Addition{{Name: "a"}, {Name: "a"}}},
	} {
		if err := Validate(bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

func TestDependencyWithHealthcheckIsGated(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image:       "mariadb:11",
		Healthcheck: &Healthcheck{Test: "healthcheck.sh --connect"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(Spec{Image: "redis:7"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(Spec{
		Image:     "wordpress:6",
		Additions: []Addition{{Name: "mariadb"}, {Name: "redis"}},
	}); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(filepath.Join(store.root, "services", "wordpress", "mariadb.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(database), "condition: service_healthy") {
		t.Fatalf("health-gated edge missing:\n%s", database)
	}
	cache, err := os.ReadFile(filepath.Join(store.root, "services", "wordpress", "redis.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cache), "condition:") {
		t.Fatalf("unhealthy dependency was gated:\n%s", cache)
	}
}

func TestPathAdditionDependsOnTheLastSegment(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{Image: "mariadb:11"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(Spec{
		Image:     "wordpress:6",
		Additions: []Addition{{Name: "database/mariadb"}},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(store.root, "services", "wordpress", "database", "mariadb.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "- mariadb") {
		t.Fatalf("edge missing:\n%s", content)
	}
}

func TestBuildFields(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Create(Spec{
		Image: "app:1",
		Build: &Build{
			Context:            "https://github.com/acme/app.git#main",
			Dockerfile:         "Dockerfile.prod",
			Target:             "runtime",
			Args:               map[string]string{"NODE_ENV": "production"},
			CacheFrom:          []string{"type=gha"},
			CacheTo:            []string{"type=gha,mode=max"},
			Platforms:          []string{"linux/amd64"},
			Tags:               []string{"app:edge"},
			Network:            "host",
			ShmSize:            "1gb",
			Pull:               true,
			NoCache:            true,
			SSH:                []string{"default"},
			AdditionalContexts: map[string]string{"config": "./config"},
			ExtraHosts:         []string{"db=10.0.0.5"},
			Entitlements:       []string{"network.host"},
			Isolation:          "default",
			Ulimits:            map[string]int{"nofile": 1024},
		},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read("app")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"context: https://github.com/acme/app.git#main", "dockerfile: Dockerfile.prod",
		"target: runtime", "NODE_ENV: production", "type=gha", "linux/amd64", "app:edge",
		"network: host", "shm_size: 1gb", "pull: true", "no_cache: true", "ssh:", "- default",
		"additional_contexts:", "config: ./config", "extra_hosts:", "entitlements:",
		"isolation: default", "ulimits:", "nofile: 1024",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("%q missing:\n%s", want, content)
		}
	}
	spec, err := store.Spec("app")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Build == nil || spec.Build.Target != "runtime" || spec.Build.Args["NODE_ENV"] != "production" || spec.Build.Ulimits["nofile"] != 1024 {
		t.Fatalf("build = %+v", spec.Build)
	}
}
