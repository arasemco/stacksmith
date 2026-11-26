package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Name        string            `json:"name"`
	Image       string            `json:"image,omitempty"`
	Build       *Build            `json:"build,omitempty"`
	Restart     string            `json:"restart,omitempty"`
	PullPolicy  string            `json:"pull_policy,omitempty"`
	Command     string            `json:"command,omitempty"`
	Ports       []string          `json:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Healthcheck *Healthcheck      `json:"healthcheck,omitempty"`
	Configs     []Config          `json:"configs,omitempty"`
	Secrets     []Secret          `json:"secrets,omitempty"`
	Profiles    []string          `json:"profiles,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Deploy      *Deploy           `json:"deploy,omitempty"`
	Additions   []Addition        `json:"additions,omitempty"`
}

type Addition struct {
	Name        string            `json:"name"`
	Depend      *bool             `json:"depend,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Configs     []Config          `json:"configs,omitempty"`
	Secrets     []Secret          `json:"secrets,omitempty"`
}

type Deploy struct {
	CPUs     string `json:"cpus,omitempty"`
	Memory   string `json:"memory,omitempty"`
	Replicas int    `json:"replicas,omitempty"`
}

type Secret struct {
	Source      string `json:"source"`
	Type        string `json:"type,omitempty"`
	Target      string `json:"target,omitempty"`
	File        string `json:"file,omitempty"`
	Environment string `json:"environment,omitempty"`
}

type Config struct {
	Source  string `json:"source"`
	Type    string `json:"type,omitempty"`
	Target  string `json:"target,omitempty"`
	Content string `json:"content,omitempty"`
	File    string `json:"file,omitempty"`
}

type Build struct {
	Context            string            `json:"context"`
	Dockerfile         string            `json:"dockerfile,omitempty"`
	DockerfileInline   string            `json:"dockerfile_inline,omitempty"`
	Target             string            `json:"target,omitempty"`
	Args               map[string]string `json:"args,omitempty"`
	CacheFrom          []string          `json:"cache_from,omitempty"`
	CacheTo            []string          `json:"cache_to,omitempty"`
	Platforms          []string          `json:"platforms,omitempty"`
	Tags               []string          `json:"tags,omitempty"`
	Network            string            `json:"network,omitempty"`
	ShmSize            string            `json:"shm_size,omitempty"`
	Pull               bool              `json:"pull,omitempty"`
	NoCache            bool              `json:"no_cache,omitempty"`
	SSH                []string          `json:"ssh,omitempty"`
	Secrets            []string          `json:"secrets,omitempty"`
	AdditionalContexts map[string]string `json:"additional_contexts,omitempty"`
	ExtraHosts         []string          `json:"extra_hosts,omitempty"`
	Entitlements       []string          `json:"entitlements,omitempty"`
	Isolation          string            `json:"isolation,omitempty"`
	Ulimits            map[string]int    `json:"ulimits,omitempty"`
}

type Healthcheck struct {
	Test        string `json:"test,omitempty"`
	Interval    string `json:"interval,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	Retries     int    `json:"retries,omitempty"`
	StartPeriod string `json:"start_period,omitempty"`
}

type Entry struct {
	Name  string   `json:"name"`
	Path  string   `json:"path"`
	Files []string `json:"files,omitempty"`
}

var ErrExists = errors.New("service already exists")

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type Store struct{ root string }

func NewStore(root string) *Store { return &Store{root: root} }

func (s *Store) file(name string) string {
	return filepath.Join(s.root, "services", name, "docker-compose.yml")
}

func (s *Store) Create(spec Spec) (Entry, error) {
	name := strings.TrimSpace(spec.Name)
	image := ImageName(spec.Image)
	if image == "" {
		return Entry{}, fmt.Errorf("service needs an image")
	}
	if name != "" && name != image {
		return Entry{}, fmt.Errorf("service name must be %q, the image name", image)
	}
	name = image
	if !namePattern.MatchString(name) {
		return Entry{}, fmt.Errorf("service name %q must be one word (letters, digits, . _ -)", spec.Name)
	}
	path := s.file(name)
	if _, err := os.Stat(path); err == nil {
		return Entry{}, ErrExists
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Entry{}, err
	}
	spec.Name = name
	text, err := Render(spec)
	if err != nil {
		return Entry{}, err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return Entry{}, err
	}
	if err := s.writeAdditions(name, spec.Additions); err != nil {
		return Entry{}, err
	}
	return Entry{Name: name, Path: relative(name)}, nil
}

func (s *Store) layerFile(name, layer string) (string, error) {
	rel, err := layerPath(name, layer)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, rel), nil
}

func layerPath(service, layer string) (string, error) {
	clean := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(layer), ".yml"), ".yaml")
	if clean == "" {
		return "", fmt.Errorf("a layer has no name")
	}
	if strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") || strings.Contains(clean, "\\") {
		return "", fmt.Errorf("layer %q must be a relative path", layer)
	}
	return filepath.Join("services", service, clean+".yml"), nil
}

func (s *Store) writeAdditions(name string, additions []Addition) error {
	keep := map[string]bool{filepath.Join(s.root, "services", name, "docker-compose.yml"): true}
	for _, addition := range additions {
		text, err := s.renderAddition(name, addition)
		if err != nil {
			return err
		}
		file, err := s.layerFile(name, addition.Name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return err
		}
		keep[file] = true
	}
	dir := filepath.Join(s.root, "services", name)
	return filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || keep[path] {
			return nil
		}
		return os.Remove(path)
	})
}

func (s *Store) hasHealthcheck(name string) bool {
	content, err := os.ReadFile(s.file(name))
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return false
	}
	services, _ := doc["services"].(map[string]any)
	service, _ := services[name].(map[string]any)
	return service["healthcheck"] != nil
}

func (s *Store) renderAddition(service string, addition Addition) (string, error) {
	value := map[string]any{}
	if addition.Depend == nil || *addition.Depend {
		name := dependencyService(addition.Name)
		if name != "" {
			if s.hasHealthcheck(name) {
				value["depends_on"] = map[string]any{
					name: map[string]any{"condition": "service_healthy"},
				}
			} else {
				value["depends_on"] = []any{name}
			}
		}
	}
	if volumes := trimmed(addition.Volumes); len(volumes) > 0 {
		value["volumes"] = volumes
	}
	labels := map[string]any{}
	for key, label := range addition.Labels {
		if key = strings.TrimSpace(key); key != "" {
			labels[key] = blockValue(label)
		}
	}
	if len(labels) > 0 {
		value["labels"] = labels
	}
	environment := map[string]any{}
	for key, item := range addition.Environment {
		if key = strings.TrimSpace(key); key != "" {
			environment[key] = blockValue(item)
		}
	}
	if len(environment) > 0 {
		value["environment"] = environment
	}
	configs := map[string]any{}
	refs := []any{}
	for _, config := range addition.Configs {
		source := strings.TrimSpace(config.Source)
		if source == "" {
			continue
		}
		ref := map[string]any{"source": source}
		if target := strings.TrimSpace(config.Target); target != "" {
			ref["target"] = target
		}
		refs = append(refs, ref)
		switch strings.TrimSpace(config.Type) {
		case "external":
		case "file":
			configs[source] = map[string]any{"file": strings.TrimSpace(config.File)}
		default:
			configs[source] = map[string]any{"content": blockValue(config.Content)}
		}
	}
	if len(refs) > 0 {
		value["configs"] = refs
	}
	secrets := map[string]any{}
	secretRefs := []any{}
	for _, secret := range addition.Secrets {
		source := strings.TrimSpace(secret.Source)
		if source == "" {
			continue
		}
		ref := map[string]any{"source": source}
		if target := strings.TrimSpace(secret.Target); target != "" {
			ref["target"] = target
		}
		secretRefs = append(secretRefs, ref)
		switch strings.TrimSpace(secret.Type) {
		case "external":
		case "environment":
			secrets[source] = map[string]any{"environment": strings.TrimSpace(secret.Environment)}
		default:
			secrets[source] = map[string]any{"file": strings.TrimSpace(secret.File)}
		}
	}
	if len(secretRefs) > 0 {
		value["secrets"] = secretRefs
	}

	doc := map[string]any{"services": map[string]any{service: value}}
	if len(configs) > 0 {
		doc["configs"] = configs
	}
	if len(secrets) > 0 {
		doc["secrets"] = secrets
	}
	return renderDocument(doc)
}

func parseAddition(service, content string) (Addition, error) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return Addition{}, err
	}
	services, _ := doc["services"].(map[string]any)
	value, _ := services[service].(map[string]any)
	addition := Addition{}
	enabled := false
	if depends, ok := value["depends_on"]; ok && depends != nil {
		enabled = true
	}
	addition.Depend = &enabled
	addition.Volumes = listOf(value["volumes"])
	if labels, ok := value["labels"].(map[string]any); ok {
		addition.Labels = map[string]string{}
		for key, label := range labels {
			addition.Labels[key] = stringOf(label)
		}
	}
	if environment, ok := value["environment"].(map[string]any); ok {
		addition.Environment = map[string]string{}
		for key, item := range environment {
			addition.Environment[key] = stringOf(item)
		}
	}
	if refs, ok := value["configs"].([]any); ok {
		top, _ := doc["configs"].(map[string]any)
		for _, ref := range refs {
			config := Config{Source: stringOf(ref.(map[string]any)["source"]), Target: stringOf(ref.(map[string]any)["target"])}
			if entry, ok := top[config.Source].(map[string]any); ok {
				if file := stringOf(entry["file"]); file != "" {
					config.Type, config.File = "file", file
				} else {
					config.Type, config.Content = "content", stringOf(entry["content"])
				}
			} else {
				config.Type = "external"
			}
			addition.Configs = append(addition.Configs, config)
		}
	}
	if refs, ok := value["secrets"].([]any); ok {
		top, _ := doc["secrets"].(map[string]any)
		for _, ref := range refs {
			secret := Secret{Source: stringOf(ref.(map[string]any)["source"]), Target: stringOf(ref.(map[string]any)["target"])}
			if entry, ok := top[secret.Source].(map[string]any); ok {
				if environment := stringOf(entry["environment"]); environment != "" {
					secret.Type, secret.Environment = "environment", environment
				} else {
					secret.Type, secret.File = "file", stringOf(entry["file"])
				}
			} else {
				secret.Type = "external"
			}
			addition.Secrets = append(addition.Secrets, secret)
		}
	}
	return addition, nil
}

func (s *Store) readAdditions(name string) ([]Addition, error) {
	dir := filepath.Join(s.root, "services", name)
	additions := []Addition{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path == filepath.Join(dir, "docker-compose.yml") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		addition, err := parseAddition(name, string(content))
		if err != nil {
			return err
		}
		addition.Name = strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
		additions = append(additions, addition)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return additions, nil
}

func (s *Store) Update(name string, spec Spec) (Entry, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return Entry{}, fmt.Errorf("service name %q is not valid", name)
	}
	if image := ImageName(spec.Image); image != name {
		return Entry{}, fmt.Errorf("service name must be %q, the image name", name)
	}
	path := s.file(name)
	if _, err := os.Stat(path); err != nil {
		return Entry{}, err
	}
	spec.Name = name
	text, err := Render(spec)
	if err != nil {
		return Entry{}, err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return Entry{}, err
	}
	if err := s.writeAdditions(name, spec.Additions); err != nil {
		return Entry{}, err
	}
	return Entry{Name: name, Path: relative(name)}, nil
}

func (s *Store) Delete(name string) error {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return fmt.Errorf("service name %q is not valid", name)
	}
	dir := filepath.Join(s.root, "services", name)
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (s *Store) files(name string) ([]string, error) {
	dir := filepath.Join(s.root, "services", name)
	files := []string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func (s *Store) ReadFile(name, path string) (string, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("service name %q is not valid", name)
	}
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return "", fmt.Errorf("file %q is not valid", path)
	}
	content, err := os.ReadFile(filepath.Join(s.root, "services", name, filepath.FromSlash(path)))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *Store) Spec(name string) (Spec, error) {
	content, err := s.Read(name)
	if err != nil {
		return Spec{}, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return Spec{}, err
	}
	services, _ := doc["services"].(map[string]any)
	service, _ := services[name].(map[string]any)
	spec := Spec{Name: name, Image: name}
	if build, ok := service["build"].(map[string]any); ok {
		b := &Build{Context: stringOf(build["context"])}
		b.Dockerfile = stringOf(build["dockerfile"])
		b.DockerfileInline = stringOf(build["dockerfile_inline"])
		b.Target = stringOf(build["target"])
		b.Network = stringOf(build["network"])
		b.ShmSize = stringOf(build["shm_size"])
		b.Isolation = stringOf(build["isolation"])
		b.Pull, _ = build["pull"].(bool)
		b.NoCache, _ = build["no_cache"].(bool)
		b.CacheFrom = listOf(build["cache_from"])
		b.CacheTo = listOf(build["cache_to"])
		b.Platforms = listOf(build["platforms"])
		b.Tags = listOf(build["tags"])
		b.SSH = listOf(build["ssh"])
		b.Secrets = listOf(build["secrets"])
		b.ExtraHosts = listOf(build["extra_hosts"])
		b.Entitlements = listOf(build["entitlements"])
		if args, ok := build["args"].(map[string]any); ok {
			b.Args = map[string]string{}
			for key, value := range args {
				b.Args[key] = stringOf(value)
			}
		}
		if contexts, ok := build["additional_contexts"].(map[string]any); ok {
			b.AdditionalContexts = map[string]string{}
			for key, value := range contexts {
				b.AdditionalContexts[key] = stringOf(value)
			}
		}
		if limits, ok := build["ulimits"].(map[string]any); ok {
			b.Ulimits = map[string]int{}
			for key, value := range limits {
				if number, ok := value.(int); ok {
					b.Ulimits[key] = number
				}
			}
		}
		spec.Build = b
	}
	spec.Restart = optionalVar(stringOf(service["restart"]), "RESTART")
	spec.PullPolicy = optionalVar(stringOf(service["pull_policy"]), "PULL_POLICY")
	spec.Command = commandText(service["command"])
	spec.Ports = listOf(service["ports"])
	spec.Volumes = listOf(service["volumes"])
	if environment, ok := service["environment"].(map[string]any); ok {
		spec.Environment = map[string]string{}
		for key, value := range environment {
			spec.Environment[key] = stringOf(value)
		}
	}
	if labels, ok := service["labels"].(map[string]any); ok {
		spec.Labels = map[string]string{}
		for key, value := range labels {
			spec.Labels[key] = stringOf(value)
		}
	}
	additions, err := s.readAdditions(name)
	if err != nil {
		return Spec{}, err
	}
	spec.Additions = additions
	spec.Profiles = listOf(service["profiles"])
	if deploy, ok := service["deploy"].(map[string]any); ok {
		d := &Deploy{}
		if replicas, ok := deploy["replicas"].(int); ok {
			d.Replicas = replicas
		}
		if resources, ok := deploy["resources"].(map[string]any); ok {
			if limits, ok := resources["limits"].(map[string]any); ok {
				d.CPUs = stringOf(limits["cpus"])
				d.Memory = stringOf(limits["memory"])
			}
		}
		spec.Deploy = d
	}
	if refs, ok := service["secrets"].([]any); ok {
		top, _ := doc["secrets"].(map[string]any)
		for _, ref := range refs {
			switch typed := ref.(type) {
			case string:
				spec.Secrets = append(spec.Secrets, Secret{Source: typed})
			case map[string]any:
				secret := Secret{Source: stringOf(typed["source"]), Target: stringOf(typed["target"])}
				if entry, ok := top[secret.Source].(map[string]any); ok {
					if environment := stringOf(entry["environment"]); environment != "" {
						secret.Type = "environment"
						secret.Environment = environment
					} else if file := stringOf(entry["file"]); file != "" {
						secret.Type = "file"
						secret.File = file
					}
				} else {
					secret.Type = "external"
				}
				spec.Secrets = append(spec.Secrets, secret)
			}
		}
	}
	if refs, ok := service["configs"].([]any); ok {
		top, _ := doc["configs"].(map[string]any)
		for _, ref := range refs {
			switch typed := ref.(type) {
			case string:
				spec.Configs = append(spec.Configs, Config{Source: typed})
			case map[string]any:
				config := Config{Source: stringOf(typed["source"]), Target: stringOf(typed["target"])}
				if entry, ok := top[config.Source].(map[string]any); ok {
					if file := stringOf(entry["file"]); file != "" {
						config.Type = "file"
						config.File = file
					} else {
						config.Type = "content"
						config.Content = stringOf(entry["content"])
					}
				} else {
					config.Type = "external"
				}
				spec.Configs = append(spec.Configs, config)
			}
		}
	}
	if check, ok := service["healthcheck"].(map[string]any); ok {
		hc := &Healthcheck{}
		switch test := check["test"].(type) {
		case string:
			hc.Test = test
		case []any:
			if len(test) == 2 && stringOf(test[0]) == "CMD-SHELL" {
				hc.Test = stringOf(test[1])
			} else if data, err := json.Marshal(test); err == nil {
				hc.Test = string(data)
			}
		}
		hc.Interval = stringOf(check["interval"])
		hc.Timeout = stringOf(check["timeout"])
		hc.StartPeriod = stringOf(check["start_period"])
		if retries, ok := check["retries"].(int); ok {
			hc.Retries = retries
		}
		spec.Healthcheck = hc
	}
	return spec, nil
}

func stringOf(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func listOf(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, stringOf(item))
	}
	return out
}

func commandText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		data, err := json.Marshal(typed)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

func (s *Store) List() ([]Entry, error) {
	dir := filepath.Join(s.root, "services")
	items, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		name := item.Name()
		if _, err := os.Stat(s.file(name)); err != nil {
			continue
		}
		files, err := s.files(name)
		if err != nil {
			return nil, err
		}
		entries = append(entries, Entry{Name: name, Path: relative(name), Files: files})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func (s *Store) Read(name string) (string, error) {
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("service name %q is not valid", name)
	}
	data, err := os.ReadFile(s.file(name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func relative(name string) string {
	return "services/" + name + "/docker-compose.yml"
}

func Render(spec Spec) (string, error) {
	service := map[string]any{}
	if image := strings.TrimSpace(spec.Image); image != "" {
		repo := ImageName(image)
		tag := tagVariable(repo)
		service["image"] = repo + ":${" + tag + ":?" + tag + " is required}"
	}
	if build := spec.Build; build != nil {
		section := map[string]any{}
		if context := strings.TrimSpace(build.Context); context != "" {
			section["context"] = context
		}
		for key, value := range map[string]string{
			"dockerfile":        build.Dockerfile,
			"dockerfile_inline": build.DockerfileInline,
			"target":            build.Target,
			"network":           build.Network,
			"shm_size":          build.ShmSize,
			"isolation":         build.Isolation,
		} {
			if value = strings.TrimSpace(value); value != "" {
				section[key] = value
			}
		}
		if args := trimmedMap(build.Args); len(args) > 0 {
			section["args"] = args
		}
		for key, values := range map[string][]string{
			"cache_from":   build.CacheFrom,
			"cache_to":     build.CacheTo,
			"platforms":    build.Platforms,
			"tags":         build.Tags,
			"ssh":          build.SSH,
			"secrets":      build.Secrets,
			"extra_hosts":  build.ExtraHosts,
			"entitlements": build.Entitlements,
		} {
			if list := trimmed(values); len(list) > 0 {
				section[key] = list
			}
		}
		if contexts := trimmedMap(build.AdditionalContexts); len(contexts) > 0 {
			section["additional_contexts"] = contexts
		}
		if limits := trimmedIntMap(build.Ulimits); len(limits) > 0 {
			section["ulimits"] = limits
		}
		if build.Pull {
			section["pull"] = true
		}
		if build.NoCache {
			section["no_cache"] = true
		}
		if len(section) > 0 {
			service["build"] = section
		}
	}
	service["restart"] = "${RESTART:-" + strings.TrimSpace(spec.Restart) + "}"
	service["pull_policy"] = "${PULL_POLICY:-" + strings.TrimSpace(spec.PullPolicy) + "}"
	service["hostname"] = strings.TrimSpace(spec.Name)
	service["domainname"] = "${STACK_DOMAIN:?STACK_DOMAIN is required}"
	if command := strings.TrimSpace(spec.Command); command != "" {
		service["command"] = commandValue(command)
	}
	if ports := trimmed(spec.Ports); len(ports) > 0 {
		service["ports"] = ports
	}
	environment := map[string]any{}
	for key, value := range spec.Environment {
		if key = strings.TrimSpace(key); key != "" {
			environment[key] = blockValue(value)
		}
	}
	if len(environment) > 0 {
		service["environment"] = environment
	}
	if volumes := trimmed(spec.Volumes); len(volumes) > 0 {
		service["volumes"] = volumes
	}
	configs := map[string]any{}
	refs := []any{}
	for _, config := range spec.Configs {
		source := strings.TrimSpace(config.Source)
		if source == "" {
			continue
		}
		ref := map[string]any{"source": source}
		if target := strings.TrimSpace(config.Target); target != "" {
			ref["target"] = target
		}
		refs = append(refs, ref)
		switch strings.TrimSpace(config.Type) {
		case "file":
			configs[source] = map[string]any{"file": strings.TrimSpace(config.File)}
		case "external":
		default:
			configs[source] = map[string]any{"content": blockValue(config.Content)}
		}
	}
	if len(refs) > 0 {
		service["configs"] = refs
	}
	secrets := map[string]any{}
	secretRefs := []any{}
	for _, secret := range spec.Secrets {
		source := strings.TrimSpace(secret.Source)
		if source == "" {
			continue
		}
		ref := map[string]any{"source": source}
		if target := strings.TrimSpace(secret.Target); target != "" {
			ref["target"] = target
		}
		secretRefs = append(secretRefs, ref)
		switch strings.TrimSpace(secret.Type) {
		case "file":
			secrets[source] = map[string]any{"file": strings.TrimSpace(secret.File)}
		case "environment":
			secrets[source] = map[string]any{"environment": strings.TrimSpace(secret.Environment)}
		case "external":
		default:
			secrets[source] = map[string]any{"file": strings.TrimSpace(secret.File)}
		}
	}
	if len(secretRefs) > 0 {
		service["secrets"] = secretRefs
	}
	labels := map[string]any{}
	for key, value := range spec.Labels {
		if key = strings.TrimSpace(key); key != "" {
			labels[key] = blockValue(value)
		}
	}
	if len(labels) > 0 {
		service["labels"] = labels
	}
	if profiles := trimmed(spec.Profiles); len(profiles) > 0 {
		service["profiles"] = profiles
	}
	if deploy := spec.Deploy; deploy != nil {
		section := map[string]any{}
		limits := map[string]any{}
		if cpus := strings.TrimSpace(deploy.CPUs); cpus != "" {
			limits["cpus"] = cpus
		}
		if memory := strings.TrimSpace(deploy.Memory); memory != "" {
			limits["memory"] = memory
		}
		if len(limits) > 0 {
			section["resources"] = map[string]any{"limits": limits}
		}
		if deploy.Replicas > 0 {
			section["replicas"] = deploy.Replicas
		}
		if len(section) > 0 {
			service["deploy"] = section
		}
	}
	if hc := spec.Healthcheck; hc != nil && strings.TrimSpace(hc.Test) != "" {
		check := map[string]any{"test": healthTest(hc.Test)}
		if interval := strings.TrimSpace(hc.Interval); interval != "" {
			check["interval"] = interval
		}
		if timeout := strings.TrimSpace(hc.Timeout); timeout != "" {
			check["timeout"] = timeout
		}
		if hc.Retries > 0 {
			check["retries"] = hc.Retries
		}
		if period := strings.TrimSpace(hc.StartPeriod); period != "" {
			check["start_period"] = period
		}
		service["healthcheck"] = check
	}

	doc := map[string]any{
		"services": map[string]any{strings.TrimSpace(spec.Name): service},
	}
	if len(configs) > 0 {
		doc["configs"] = configs
	}
	if len(secrets) > 0 {
		doc["secrets"] = secrets
	}
	return renderDocument(doc)
}

func dependencyService(name string) string {
	parts := strings.Split(strings.TrimSpace(name), "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

func renderDocument(doc map[string]any) (string, error) {
	var out strings.Builder
	for _, key := range []string{"services", "configs", "secrets"} {
		section, ok := doc[key]
		if !ok {
			continue
		}
		var chunk strings.Builder
		encoder := yaml.NewEncoder(&chunk)
		encoder.SetIndent(2)
		if err := encoder.Encode(map[string]any{key: section}); err != nil {
			return "", err
		}
		if err := encoder.Close(); err != nil {
			return "", err
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(chunk.String())
	}
	return spaceServiceBlocks(out.String()), nil
}

func spaceServiceBlocks(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	out := make([]string, 0, len(lines))
	blockIndent := -1
	lastWasBlock := false
	for _, line := range lines {
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if blockIndent >= 0 {
			if trimmed == "" || indent > blockIndent {
				out = append(out, line)
				continue
			}
			blockIndent = -1
		}
		if strings.HasSuffix(trimmed, "|") || strings.HasSuffix(trimmed, "|-") || strings.HasSuffix(trimmed, ">") || strings.HasSuffix(trimmed, ">-") {
			blockIndent = indent
		}
		if indent == 4 && !strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, ":") {
			isBlock := strings.HasSuffix(trimmed, ":")
			previous := ""
			if len(out) > 0 {
				previous = out[len(out)-1]
			}
			previousIndent := len(previous) - len(strings.TrimLeft(previous, " "))
			if previous != "" && !(previousIndent == 2 && strings.HasSuffix(strings.TrimSpace(previous), ":")) {
				if lastWasBlock || isBlock {
					out = append(out, "")
				}
			}
			lastWasBlock = isBlock
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n") + "\n"
}

func commandValue(command string) any {
	if strings.HasPrefix(command, "[") {
		var list []any
		if err := json.Unmarshal([]byte(command), &list); err == nil {
			return list
		}
	}
	return command
}

func trimmedMap(values map[string]string) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		if key = strings.TrimSpace(key); key != "" {
			out[key] = value
		}
	}
	return out
}

func trimmedIntMap(values map[string]int) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		if key = strings.TrimSpace(key); key != "" && value > 0 {
			out[key] = value
		}
	}
	return out
}

func trimmed(values []string) []any {
	out := []any{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// ImageName is the repository name of an image: the last path segment without
// its tag or digest.
func ImageName(image string) string {
	name := strings.TrimSpace(image)
	if at := strings.Index(name, "@"); at >= 0 {
		name = name[:at]
	}
	if colon := strings.LastIndex(name, ":"); colon > strings.LastIndex(name, "/") {
		name = name[:colon]
	}
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		name = name[slash+1:]
	}
	return name
}

func blockValue(value string) string {
	content := dedent(value)
	if strings.Contains(content, "\n") && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content
}

func dedent(value string) string {
	if !strings.Contains(value, "\n") {
		return value
	}
	lines := strings.Split(value, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		width := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || width < indent {
			indent = width
		}
	}
	if indent > 0 {
		for i, line := range lines {
			if len(line) >= indent {
				lines[i] = line[indent:]
			}
		}
	}
	return strings.Join(lines, "\n")
}

func optionalVar(value, name string) string {
	prefix := "${" + name + ":-"
	if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, "}") {
		return strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}")
	}
	return value
}

func healthTest(test string) any {
	if strings.HasPrefix(test, "[") {
		var list []any
		if err := json.Unmarshal([]byte(test), &list); err == nil {
			return list
		}
	}
	return []any{"CMD-SHELL", strings.TrimSpace(test)}
}

func tagVariable(repo string) string {
	var out strings.Builder
	for _, char := range strings.ToUpper(repo) {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			out.WriteRune(char)
		} else {
			out.WriteRune('_')
		}
	}
	return out.String() + "_TAG"
}

func Validate(spec Spec) error {
	secretSeen := map[string]bool{}
	for _, secret := range spec.Secrets {
		source := strings.TrimSpace(secret.Source)
		if source == "" {
			return fmt.Errorf("a secret row has no source name")
		}
		if secretSeen[source] {
			return fmt.Errorf("secret %q is defined twice", source)
		}
		secretSeen[source] = true
		switch strings.TrimSpace(secret.Type) {
		case "file":
			if strings.TrimSpace(secret.File) == "" {
				return fmt.Errorf("secret %q has no file", source)
			}
		case "environment":
			if strings.TrimSpace(secret.Environment) == "" {
				return fmt.Errorf("secret %q has no environment variable", source)
			}
		case "external":
		default:
			return fmt.Errorf("secret %q has an unknown type %q", source, secret.Type)
		}
	}
	additionSeen := map[string]bool{}
	for _, addition := range spec.Additions {
		if addition.Depend == nil || *addition.Depend {
			if !namePattern.MatchString(dependencyService(addition.Name)) {
				return fmt.Errorf("addition %q has no service name to depend on, or untick depend", addition.Name)
			}
		}
		path, err := layerPath(spec.Name, addition.Name)
		if err != nil {
			return err
		}
		if additionSeen[path] {
			return fmt.Errorf("addition %q is defined twice", addition.Name)
		}
		additionSeen[path] = true
	}
	seen := map[string]bool{}
	for _, config := range spec.Configs {
		source := strings.TrimSpace(config.Source)
		if source == "" {
			return fmt.Errorf("a config row has no source name")
		}
		if seen[source] {
			return fmt.Errorf("config %q is defined twice", source)
		}
		seen[source] = true
		switch strings.TrimSpace(config.Type) {
		case "", "content":
			if strings.TrimSpace(config.Content) == "" {
				return fmt.Errorf("config %q has no content", source)
			}
		case "file":
			if strings.TrimSpace(config.File) == "" {
				return fmt.Errorf("config %q has no file", source)
			}
		case "external":
		default:
			return fmt.Errorf("config %q has an unknown type %q", source, config.Type)
		}
	}
	return nil
}
