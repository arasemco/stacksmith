import hljs from "highlight.js/lib/core";
import yaml from "highlight.js/lib/languages/yaml";
import "highlight.js/styles/tokyo-night-dark.css";
import "./style.css";
import { catalog, createService, createStack, deleteService, deleteStack, listServices, listStacks, readService, readServiceFile, readStack, updateService, updateStack, type Addition, type Build, type Config, type Entry, type Secret, type ServiceRef, type ServiceSpec } from "./api";
import { mountAdditions } from "./addition";

hljs.registerLanguage("yaml", yaml);

type Row = { a: string; b: string };
type Rows = { ports: Row[]; env: Row[]; volumes: Row[]; labels: Row[]; buildArgs: Row[]; buildContexts: Row[]; buildUlimits: Row[] };

const $ = <T extends HTMLElement>(id: string): T => {
  const element = document.getElementById(id);
  if (!element) throw new Error(`missing #${id}`);
  return element as T;
};

const els = {
  status: $<HTMLElement>("status"),
  image: $<HTMLInputElement>("image"),
  build: $<HTMLInputElement>("build"),
  buildDockerfile: $<HTMLInputElement>("build-dockerfile"),
  buildTarget: $<HTMLInputElement>("build-target"),
  buildNetwork: $<HTMLInputElement>("build-network"),
  buildShm: $<HTMLInputElement>("build-shm"),
  buildIsolation: $<HTMLInputElement>("build-isolation"),
  buildPlatforms: $<HTMLInputElement>("build-platforms"),
  buildCacheFrom: $<HTMLInputElement>("build-cache-from"),
  buildCacheTo: $<HTMLInputElement>("build-cache-to"),
  buildTags: $<HTMLInputElement>("build-tags"),
  buildSsh: $<HTMLInputElement>("build-ssh"),
  buildExtraHosts: $<HTMLInputElement>("build-extra-hosts"),
  buildEntitlements: $<HTMLInputElement>("build-entitlements"),
  buildInline: $<HTMLTextAreaElement>("build-inline"),
  buildPull: $<HTMLInputElement>("build-pull"),
  buildNoCache: $<HTMLInputElement>("build-no-cache"),
  restart: $<HTMLSelectElement>("restart"),
  pull: $<HTMLSelectElement>("pull"),
  command: $<HTMLInputElement>("command"),
  deployCpus: $<HTMLInputElement>("deploy-cpus"),
  deployMemory: $<HTMLInputElement>("deploy-memory"),
  deployReplicas: $<HTMLInputElement>("deploy-replicas"),
  profiles: $<HTMLInputElement>("profiles"),
  additions: $<HTMLUListElement>("additions"),
  addAddition: $<HTMLButtonElement>("add-addition"),
  additionEditor: $<HTMLElement>("addition-editor"),
  hcTest: $<HTMLInputElement>("hc-test"),
  hcInterval: $<HTMLInputElement>("hc-interval"),
  hcTimeout: $<HTMLInputElement>("hc-timeout"),
  hcRetries: $<HTMLInputElement>("hc-retries"),
  hcStart: $<HTMLInputElement>("hc-start"),
  save: $<HTMLButtonElement>("save"),
  new: $<HTMLButtonElement>("new"),
  files: $<HTMLElement>("files"),
  code: $<HTMLElement>("code"),
  activePath: $<HTMLElement>("active-path"),
  copy: $<HTMLButtonElement>("copy"),
  modes: $<HTMLElement>("modes"),
  servicePanel: $<HTMLElement>("service-panel"),
  stackPanel: $<HTMLElement>("stack-panel"),
  stackName: $<HTMLInputElement>("stack-name"),
  stackTemplates: $<HTMLUListElement>("stack-templates"),
  stackPicker: $<HTMLSelectElement>("stack-picker"),
  stackAdd: $<HTMLButtonElement>("stack-add"),
  stackSave: $<HTMLButtonElement>("stack-save"),
  stackNew: $<HTMLButtonElement>("stack-new"),
  download: $<HTMLButtonElement>("download"),
};

const configs: Config[] = [];
const secrets: Secret[] = [];
const additions: Addition[] = [];
let refreshAdditions: () => void = () => undefined;

const rows: Rows = {
  ports: [{ a: "3000", b: "3000" }],
  env: [{ a: "NODE_ENV", b: "production" }],
  volumes: [],
  labels: [],
  buildArgs: [],
  buildContexts: [],
  buildUlimits: [],
};

let entries: Entry[] = [];
let stacks: Entry[] = [];
let mode: "service" | "stack" = "service";
let stackEditing: string | null = null;
let stackDirect: string[] = [];
let stackFeatures: string[] = [];
let catalogRefs: ServiceRef[] = [];
let selected: string | null = null;
let editing: string | null = null;
let text = "";

function setStatus(message: string, kind: "ok" | "error" | "" = ""): void {
  els.status.textContent = message;
  els.status.title = message;
  els.status.className = `status ${kind}`;
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function fieldRow(
  value: Row,
  placeholder: [string, string],
  onChange: () => void,
  onRemove: () => void,
  multiline = false,
): HTMLElement {
  const row = document.createElement("div");
  row.className = "row-line";
  const first = document.createElement("input");
  first.value = value.a;
  first.placeholder = placeholder[0];
  first.spellcheck = false;
  const second = multiline ? document.createElement("textarea") : document.createElement("input");
  second.value = value.b;
  second.placeholder = placeholder[1];
  second.spellcheck = false;
  if (multiline) {
    (second as HTMLTextAreaElement).rows = 1;
  }
  first.addEventListener("input", () => {
    value.a = first.value;
    onChange();
  });
  second.addEventListener("input", () => {
    value.b = second.value;
    onChange();
  });
  const remove = document.createElement("button");
  remove.className = "icon";
  remove.textContent = "✕";
  remove.title = "Remove";
  remove.addEventListener("click", onRemove);
  row.append(first, separator(), second, remove);
  return row;
}

function separator(): HTMLElement {
  const span = document.createElement("span");
  span.className = "sep";
  span.textContent = ":";
  return span;
}

function renderRows(key: keyof Rows, placeholders: [string, string]): void {
  const container = $(key);
  container.replaceChildren();
  rows[key].forEach((row, index) => {
    container.append(
      fieldRow(
        row,
        placeholders,
        () => undefined,
        () => {
          rows[key].splice(index, 1);
          renderRows(key, placeholders);
        },
        key === "env" || key === "labels",
      ),
    );
  });
}

function addRow(button: HTMLButtonElement): void {
  const key = button.dataset.add as keyof Rows;
  rows[key].push({ a: "", b: "" });
  renderRows(key, placeholdersFor(key));
}

function placeholdersFor(key: keyof Rows): [string, string] {
  if (key === "env" || key === "labels" || key === "buildArgs") return ["key", "value"];
  if (key === "buildContexts") return ["name", "./path"];
  if (key === "buildUlimits") return ["nofile", "1024"];
  if (key === "volumes") return ["Host / Named Vol", "/container/path"];
  return ["Host", "Container"];
}

function pair(row: Row): string {
  const second = row.b.trim();
  const first = row.a.trim();
  if (second === "") return "";
  return first === "" ? second : `${first}:${second}`;
}

function buildSpec(): ServiceSpec {
  const image = els.image.value.trim();
  const name = imageName(image);
  if (name === "") throw new Error("image is required");
  const environment: Record<string, string> = {};
  for (const row of rows.env) {
    if (row.a.trim() !== "") environment[row.a.trim()] = row.b;
  }
  const labels: Record<string, string> = {};
  for (const row of rows.labels) {
    if (row.a.trim() !== "") labels[row.a.trim()] = row.b;
  }
  const ports = rows.ports.map(pair).filter(Boolean);
  const volumes = rows.volumes.map(pair).filter(Boolean);
  return {
    name,
    image,
    ...(buildSection() ? { build: buildSection() } : {}),
    ...(els.restart.value ? { restart: els.restart.value } : {}),
    ...(els.pull.value ? { pull_policy: els.pull.value } : {}),
    ...(els.command.value.trim() ? { command: els.command.value.trim() } : {}),
    ...(ports.length ? { ports } : {}),
    ...(Object.keys(environment).length ? { environment } : {}),
    ...(Object.keys(labels).length ? { labels } : {}),
    ...(volumes.length ? { volumes } : {}),
    ...(els.hcTest.value.trim() ? { healthcheck: healthcheck() } : {}),
    ...(deploy() ? { deploy: deploy() } : {}),
    ...(additions.some((addition) => addition.name.trim())
      ? { additions: additions.filter((addition) => addition.name.trim()) }
      : {}),
    ...(profileList().length ? { profiles: profileList() } : {}),
    ...(secrets.some((secret) => secret.source.trim())
      ? {
          secrets: secrets
            .filter((secret) => secret.source.trim())
            .map((secret) => ({
              source: secret.source.trim(),
              type: secret.type ?? "file",
              ...(secret.target?.trim() ? { target: secret.target.trim() } : {}),
              ...(secret.type === "environment" ? { environment: secret.environment ?? "" } : {}),
              ...((secret.type ?? "file") === "file" ? { file: secret.file ?? "" } : {}),
            })),
        }
      : {}),
    ...(configs.some((config) => config.source.trim())
      ? {
          configs: configs
            .filter((config) => config.source.trim())
            .map((config) => ({
              source: config.source.trim(),
              type: config.type ?? "content",
              ...(config.target?.trim() ? { target: config.target.trim() } : {}),
              ...(config.type === "file" ? { file: config.file ?? "" } : {}),
              ...((config.type ?? "content") === "content" ? { content: config.content ?? "" } : {}),
            })),
        }
      : {}),
  };
}

function deploy() {
  const replicas = Number(els.deployReplicas.value);
  const section = {
    ...(els.deployCpus.value.trim() ? { cpus: els.deployCpus.value.trim() } : {}),
    ...(els.deployMemory.value.trim() ? { memory: els.deployMemory.value.trim() } : {}),
    ...(Number.isFinite(replicas) && replicas > 0 ? { replicas } : {}),
  };
  return Object.keys(section).length ? section : undefined;
}

function profileList(): string[] {
  return els.profiles.value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function listOf(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function buildSection(): Build | undefined {
  const args = Object.fromEntries(
    rows.buildArgs.filter((row) => row.a.trim()).map((row) => [row.a.trim(), row.b]),
  );
  const additional_contexts = Object.fromEntries(
    rows.buildContexts.filter((row) => row.a.trim()).map((row) => [row.a.trim(), row.b.trim()]),
  );
  const ulimits = Object.fromEntries(
    rows.buildUlimits
      .filter((row) => row.a.trim() && Number(row.b) > 0)
      .map((row) => [row.a.trim(), Number(row.b)]),
  );
  const build: Build = {
    ...(els.build.value.trim() ? { context: els.build.value.trim() } : {}),
    ...(els.buildDockerfile.value.trim() ? { dockerfile: els.buildDockerfile.value.trim() } : {}),
    ...(els.buildTarget.value.trim() ? { target: els.buildTarget.value.trim() } : {}),
    ...(els.buildNetwork.value.trim() ? { network: els.buildNetwork.value.trim() } : {}),
    ...(els.buildShm.value.trim() ? { shm_size: els.buildShm.value.trim() } : {}),
    ...(els.buildIsolation.value.trim() ? { isolation: els.buildIsolation.value.trim() } : {}),
    ...(listOf(els.buildPlatforms.value).length ? { platforms: listOf(els.buildPlatforms.value) } : {}),
    ...(listOf(els.buildCacheFrom.value).length ? { cache_from: listOf(els.buildCacheFrom.value) } : {}),
    ...(listOf(els.buildCacheTo.value).length ? { cache_to: listOf(els.buildCacheTo.value) } : {}),
    ...(listOf(els.buildTags.value).length ? { tags: listOf(els.buildTags.value) } : {}),
    ...(listOf(els.buildSsh.value).length ? { ssh: listOf(els.buildSsh.value) } : {}),
    ...(listOf(els.buildExtraHosts.value).length ? { extra_hosts: listOf(els.buildExtraHosts.value) } : {}),
    ...(listOf(els.buildEntitlements.value).length ? { entitlements: listOf(els.buildEntitlements.value) } : {}),
    ...(els.buildInline.value.trim() ? { dockerfile_inline: els.buildInline.value } : {}),
    ...(els.buildPull.checked ? { pull: true } : {}),
    ...(els.buildNoCache.checked ? { no_cache: true } : {}),
    ...(Object.keys(args).length ? { args } : {}),
    ...(Object.keys(additional_contexts).length ? { additional_contexts } : {}),
    ...(Object.keys(ulimits).length ? { ulimits } : {}),
  };
  return Object.keys(build).length ? build : undefined;
}

function healthcheck() {
  const retries = Number(els.hcRetries.value);
  return {
    test: els.hcTest.value.trim(),
    ...(els.hcInterval.value.trim() ? { interval: els.hcInterval.value.trim() } : {}),
    ...(els.hcTimeout.value.trim() ? { timeout: els.hcTimeout.value.trim() } : {}),
    ...(Number.isFinite(retries) && retries > 0 ? { retries } : {}),
    ...(els.hcStart.value.trim() ? { start_period: els.hcStart.value.trim() } : {}),
  };
}

function dependencyName(addition: string): string {
  const parts = addition.split("/");
  return parts[parts.length - 1];
}

function stackTemplates(): string[] {
  const paths: string[] = [];
  for (const name of stackDirect) {
    paths.push(`services/${name}/docker-compose.yml`);
    for (const key of stackFeatures) {
      const [owner, addition] = key.split("::");
      if (owner === name) paths.push(`services/${owner}/${addition}.yml`);
    }
  }
  return paths;
}

function featureKey(service: string, addition: string): string {
  return `${service}::${addition}`;
}

function renderStackTemplates(): void {
  els.stackTemplates.replaceChildren();
  for (const name of stackDirect) {
    const service = catalogRefs.find((ref) => ref.name === name);
    const row = document.createElement("li");
    row.className = "stack-template";
    const label = document.createElement("span");
    label.textContent = name;
    const remove = document.createElement("button");
    remove.className = "icon";
    remove.textContent = "✕";
    remove.title = `Remove ${name}`;
    remove.addEventListener("click", () => {
      stackDirect = stackDirect.filter((item) => item !== name);
      stackFeatures = stackFeatures.filter((key) => key.split("::")[0] !== name);
      renderStackTemplates();
    });
    row.append(label, remove);
    els.stackTemplates.append(row);

    for (const addition of service?.additions ?? []) {
      const key = featureKey(name, addition.name);
      const featureRow = document.createElement("li");
      featureRow.className = "stack-template feature";
      const featureBox = document.createElement("input");
      featureBox.type = "checkbox";
      featureBox.checked = stackFeatures.includes(key);
      featureBox.addEventListener("change", () => {
        if (featureBox.checked) {
          stackFeatures = [...stackFeatures, key];
          if (addition.depend !== false) {
            const dependency = dependencyName(addition.name);
            if (dependency && !stackDirect.includes(dependency)) stackDirect = [...stackDirect, dependency];
          }
        } else {
          stackFeatures = stackFeatures.filter((item) => item !== key);
          if (addition.depend !== false) {
            const dependency = dependencyName(addition.name);
            const stillNeeded = stackFeatures.some((other) => dependencyName(other.split("::")[1] ?? "") === dependency);
            if (!stillNeeded) stackDirect = stackDirect.filter((item) => item !== dependency);
          }
        }
        renderStackTemplates();
      });
      const featureLabel = document.createElement("span");
      featureLabel.textContent =
        addition.depend === false ? addition.name : `→ ${dependencyName(addition.name)}`;
      featureRow.append(featureBox, featureLabel);
      els.stackTemplates.append(featureRow);
    }
  }
  if (stackDirect.length === 0) {
    const empty = document.createElement("li");
    empty.className = "folder";
    empty.textContent = "no services selected";
    els.stackTemplates.append(empty);
  }
  renderStackPicker();
  renderStackPreview();
}

function renderStackPreview(): void {
  if (mode !== "stack") return;
  const templates = stackTemplates();
  if (templates.length === 0) {
    clearOutput();
    return;
  }
  const name = els.stackName.value.trim();
  const lines = ["include:"];
  for (const service of stackDirect) {
    const files = [`services/${service}/docker-compose.yml`];
    for (const key of stackFeatures) {
      const [owner, addition] = key.split("::");
      if (owner === service) files.push(`services/${owner}/${addition}.yml`);
    }
    if (files.length === 1) {
      lines.push(`  - path: ../../${files[0]}`);
    } else {
      lines.push("  - path:");
      for (const file of files) lines.push(`      - ../../${file}`);
    }
  }
  text = lines.join("\n") + "\n";
  els.activePath.textContent = name ? `stacks/${name}/docker-compose.yml` : "";
  els.code.innerHTML = hljs.highlight(text, { language: "yaml" }).value;
}

function renderStackPicker(): void {
  els.stackPicker.replaceChildren();
  const available = catalogRefs.filter((service) => !stackDirect.includes(service.name));
  for (const service of available) {
    const option = document.createElement("option");
    option.value = service.name;
    option.textContent = service.name;
    els.stackPicker.append(option);
  }
  els.stackPicker.disabled = available.length === 0;
  els.stackAdd.disabled = available.length === 0;
}

function renderStackTree(): void {
  if (stacks.length === 0) {
    const empty = document.createElement("div");
    empty.className = "folder";
    empty.textContent = "no stacks stored";
    els.files.append(empty);
    return;
  }
  for (const stack of stacks) {
    const row = document.createElement("div");
    row.className = "file-row";
    const button = document.createElement("button");
    button.className = stack.name === stackEditing ? "file active" : "file";
    button.textContent = stack.name;
    button.dataset.stack = stack.name;
    const remove = document.createElement("button");
    remove.className = "icon";
    remove.textContent = "✕";
    remove.title = `Delete ${stack.name}`;
    remove.dataset.deleteStack = stack.name;
    row.append(button, remove);
    els.files.append(row);
    if (stack.name !== stackEditing) continue;
    for (const name of stackDirect) {
      const leaf = document.createElement("button");
      leaf.className = "file leaf main";
      leaf.textContent = name;
      leaf.title = `services/${name}/docker-compose.yml`;
      leaf.dataset.member = name;
      els.files.append(leaf);
    }
  }
}

function clearOutput(): void {
  text = "";
  els.activePath.textContent = "";
  els.code.innerHTML = "";
}

function switchMode(next: "service" | "stack"): void {
  mode = next;
  els.servicePanel.hidden = next !== "service";
  els.stackPanel.hidden = next !== "stack";
  for (const button of els.modes.querySelectorAll<HTMLButtonElement>("button")) {
    button.classList.toggle("active", button.dataset.mode === next);
  }
  if (next === "stack" && stackEditing === null) clearOutput();
  if (next === "service" && selected === null) clearOutput();
  renderTree();
}

async function showStack(name: string): Promise<void> {
  try {
    const stack = await readStack(name);
    switchMode("stack");
    stackEditing = name;
    const templates = [...stack.spec.templates];
    stackDirect = [];
    stackFeatures = [];
    for (const template of templates) {
      const parts = template.split("/");
      const owner = parts[1] ?? "";
      if (template.endsWith("/docker-compose.yml")) {
        if (owner && !stackDirect.includes(owner)) stackDirect.push(owner);
      } else {
        const addition = parts.slice(2).join("/").replace(/\.yml$/, "");
        if (owner && addition) stackFeatures.push(featureKey(owner, addition));
      }
    }
    els.stackName.value = name;
    text = stack.content;
    els.activePath.textContent = stack.path;
    els.code.innerHTML = hljs.highlight(stack.content, { language: "yaml" }).value;
    renderTree();
    renderStackTemplates();
    renderTreeSelection();
  } catch (err) {
    setStatus(message(err), "error");
  }
}

function renderTreeSelection(): void {
  for (const button of els.files.querySelectorAll<HTMLButtonElement>("button")) {
    if (button.dataset.stack) {
      button.classList.toggle("active", mode === "stack" && button.dataset.stack === stackEditing);
    } else {
      button.classList.toggle("active", button.dataset.name === selected && mode === "service");
    }
  }
}

async function saveStack(): Promise<void> {
  const name = els.stackName.value.trim();
  try {
    if (name === "") throw new Error("the stack needs a name");
    const templates = stackTemplates();
    if (templates.length === 0) throw new Error("the stack includes no template");
    if (stackEditing) {
      await updateStack(stackEditing, { name: stackEditing, templates });
      setStatus(`updated ${stackEditing}`, "ok");
    } else {
      await createStack({ name, templates });
      stackEditing = name;
      setStatus(`stored ${name}`, "ok");
    }
    await refresh();
    await showStack(stackEditing ?? name);
  } catch (err) {
    setStatus(message(err), "error");
  }
}

function renderConfigs(): void {
  const container = $("configs");
  container.replaceChildren();
  configs.forEach((config, index) => {
    const block = document.createElement("div");
    block.className = "config-row";
    const head = document.createElement("div");
    head.className = "config-head";
    const source = document.createElement("input");
    source.value = config.source;
    source.placeholder = "source";
    source.spellcheck = false;
    source.addEventListener("input", () => {
      config.source = source.value;
    });
    const type = document.createElement("select");
    for (const option of ["content", "file", "external"]) {
      const item = document.createElement("option");
      item.value = option;
      item.textContent = option;
      type.append(item);
    }
    type.value = config.type ?? "content";
    type.addEventListener("change", () => {
      config.type = type.value as Config["type"];
      renderConfigs();
    });
    const target = document.createElement("input");
    target.value = config.target ?? "";
    target.placeholder = "/container/path";
    target.spellcheck = false;
    target.addEventListener("input", () => {
      config.target = target.value;
    });
    const remove = document.createElement("button");
    remove.className = "icon";
    remove.textContent = "✕";
    remove.title = "Remove";
    remove.addEventListener("click", () => {
      configs.splice(index, 1);
      renderConfigs();
    });
    head.append(source, type, target, remove);
    block.append(head);
    if ((config.type ?? "content") === "content") {
      const content = document.createElement("textarea");
      content.value = config.content ?? "";
      content.placeholder = "content";
      content.spellcheck = false;
      content.rows = 3;
      content.addEventListener("input", () => {
        config.content = content.value;
      });
      block.append(content);
    } else if (config.type === "file") {
      const file = document.createElement("input");
      file.value = config.file ?? "";
      file.placeholder = "./config.conf";
      file.spellcheck = false;
      file.addEventListener("input", () => {
        config.file = file.value;
      });
      block.append(file);
    }
    container.append(block);
  });
}

function renderSecrets(): void {
  const container = $("secrets");
  container.replaceChildren();
  secrets.forEach((secret, index) => {
    const block = document.createElement("div");
    block.className = "config-row";
    const head = document.createElement("div");
    head.className = "config-head";
    const source = document.createElement("input");
    source.value = secret.source;
    source.placeholder = "source";
    source.spellcheck = false;
    source.addEventListener("input", () => {
      secret.source = source.value;
    });
    const type = document.createElement("select");
    for (const option of ["file", "environment", "external"]) {
      const item = document.createElement("option");
      item.value = option;
      item.textContent = option;
      type.append(item);
    }
    type.value = secret.type ?? "file";
    type.addEventListener("change", () => {
      secret.type = type.value as Secret["type"];
      renderSecrets();
    });
    const target = document.createElement("input");
    target.value = secret.target ?? "";
    target.placeholder = "/run/secrets/name";
    target.spellcheck = false;
    target.addEventListener("input", () => {
      secret.target = target.value;
    });
    const remove = document.createElement("button");
    remove.className = "icon";
    remove.textContent = "✕";
    remove.title = "Remove";
    remove.addEventListener("click", () => {
      secrets.splice(index, 1);
      renderSecrets();
    });
    head.append(source, type, target, remove);
    block.append(head);
    if (secret.type === "environment") {
      const variable = document.createElement("input");
      variable.value = secret.environment ?? "";
      variable.placeholder = "ENV_VAR";
      variable.spellcheck = false;
      variable.addEventListener("input", () => {
        secret.environment = variable.value;
      });
      block.append(variable);
    } else if ((secret.type ?? "file") === "file") {
      const file = document.createElement("input");
      file.value = secret.file ?? "";
      file.placeholder = "./secret.txt";
      file.spellcheck = false;
      file.addEventListener("input", () => {
        secret.file = file.value;
      });
      block.append(file);
    }
    container.append(block);
  });
}

function splitPair(value: string): Row {
  const at = value.indexOf(":");
  return at < 0 ? { a: "", b: value } : { a: value.slice(0, at), b: value.slice(at + 1) };
}

function fillForm(spec: ServiceSpec): void {
  els.image.value = spec.image ?? "";
  els.build.value = spec.build?.context ?? "";
  els.buildDockerfile.value = spec.build?.dockerfile ?? "";
  els.buildTarget.value = spec.build?.target ?? "";
  els.buildNetwork.value = spec.build?.network ?? "";
  els.buildShm.value = spec.build?.shm_size ?? "";
  els.buildIsolation.value = spec.build?.isolation ?? "";
  els.buildPlatforms.value = (spec.build?.platforms ?? []).join(", ");
  els.buildCacheFrom.value = (spec.build?.cache_from ?? []).join(", ");
  els.buildCacheTo.value = (spec.build?.cache_to ?? []).join(", ");
  els.buildTags.value = (spec.build?.tags ?? []).join(", ");
  els.buildSsh.value = (spec.build?.ssh ?? []).join(", ");
  els.buildExtraHosts.value = (spec.build?.extra_hosts ?? []).join(", ");
  els.buildEntitlements.value = (spec.build?.entitlements ?? []).join(", ");
  els.buildInline.value = spec.build?.dockerfile_inline ?? "";
  els.buildPull.checked = spec.build?.pull ?? false;
  els.buildNoCache.checked = spec.build?.no_cache ?? false;
  rows.buildArgs = Object.entries(spec.build?.args ?? {}).map(([key, value]) => ({ a: key, b: value }));
  rows.buildContexts = Object.entries(spec.build?.additional_contexts ?? {}).map(([key, value]) => ({ a: key, b: value }));
  rows.buildUlimits = Object.entries(spec.build?.ulimits ?? {}).map(([key, value]) => ({ a: key, b: String(value) }));
  els.deployCpus.value = spec.deploy?.cpus ?? "";
  els.deployMemory.value = spec.deploy?.memory ?? "";
  els.deployReplicas.value = spec.deploy?.replicas ? String(spec.deploy.replicas) : "";
  els.profiles.value = (spec.profiles ?? []).join(", ");
  els.hcTest.value = spec.healthcheck?.test ?? "";
  els.hcInterval.value = spec.healthcheck?.interval ?? "";
  els.hcTimeout.value = spec.healthcheck?.timeout ?? "";
  els.hcRetries.value = spec.healthcheck?.retries ? String(spec.healthcheck.retries) : "";
  els.hcStart.value = spec.healthcheck?.start_period ?? "";
  els.restart.value = spec.restart ?? "";
  els.pull.value = spec.pull_policy ?? "";
  els.command.value = spec.command ?? "";
  rows.ports = (spec.ports ?? []).map(splitPair);
  rows.volumes = (spec.volumes ?? []).map(splitPair);
  rows.env = Object.entries(spec.environment ?? {}).map(([key, value]) => ({ a: key, b: value }));
  rows.labels = Object.entries(spec.labels ?? {}).map(([key, value]) => ({ a: key, b: value }));
  configs.splice(0, configs.length, ...(spec.configs ?? []).map((config) => ({ ...config })));
  secrets.splice(0, secrets.length, ...(spec.secrets ?? []).map((secret) => ({ ...secret })));
  additions.splice(0, additions.length, ...(spec.additions ?? []).map((addition) => structuredClone(addition)));
  refreshAdditions();
  renderRows("ports", placeholdersFor("ports"));
  renderRows("env", placeholdersFor("env"));
  renderRows("labels", placeholdersFor("labels"));
renderRows("buildArgs", placeholdersFor("buildArgs"));
renderRows("buildContexts", placeholdersFor("buildContexts"));
renderRows("buildUlimits", placeholdersFor("buildUlimits"));
  renderRows("buildArgs", placeholdersFor("buildArgs"));
  renderRows("buildContexts", placeholdersFor("buildContexts"));
  renderRows("buildUlimits", placeholdersFor("buildUlimits"));
  renderRows("volumes", placeholdersFor("volumes"));
  renderConfigs();
  renderSecrets();
}

function resetForm(): void {
  editing = null;
  els.image.value = "";
  els.build.value = "";
  els.buildDockerfile.value = "";
  els.buildTarget.value = "";
  els.buildNetwork.value = "";
  els.buildShm.value = "";
  els.buildIsolation.value = "";
  els.buildPlatforms.value = "";
  els.buildCacheFrom.value = "";
  els.buildCacheTo.value = "";
  els.buildTags.value = "";
  els.buildSsh.value = "";
  els.buildExtraHosts.value = "";
  els.buildEntitlements.value = "";
  els.buildInline.value = "";
  els.buildPull.checked = false;
  els.buildNoCache.checked = false;
  rows.buildArgs = [];
  rows.buildContexts = [];
  rows.buildUlimits = [];
  els.deployCpus.value = "";
  els.deployMemory.value = "";
  els.deployReplicas.value = "";
  els.profiles.value = "";
  els.hcTest.value = "";
  els.hcInterval.value = "";
  els.hcTimeout.value = "";
  els.hcRetries.value = "";
  els.hcStart.value = "";
  els.build.value = "";
  els.restart.value = "unless-stopped";
  els.pull.value = "missing";
  els.command.value = "";
  rows.ports = [];
  rows.env = [];
  rows.volumes = [];
  rows.labels = [];
  configs.splice(0, configs.length);
  secrets.splice(0, secrets.length);
  additions.splice(0, additions.length);
  refreshAdditions();
  renderRows("ports", placeholdersFor("ports"));
  renderRows("env", placeholdersFor("env"));
  renderRows("labels", placeholdersFor("labels"));
renderRows("buildArgs", placeholdersFor("buildArgs"));
renderRows("buildContexts", placeholdersFor("buildContexts"));
renderRows("buildUlimits", placeholdersFor("buildUlimits"));
  renderRows("buildArgs", placeholdersFor("buildArgs"));
  renderRows("buildContexts", placeholdersFor("buildContexts"));
  renderRows("buildUlimits", placeholdersFor("buildUlimits"));
  renderRows("volumes", placeholdersFor("volumes"));
  renderConfigs();
  renderSecrets();
  els.save.textContent = "Save service";
  setStatus("");
}

async function remove(name: string): Promise<void> {
  if (!confirm(`Delete ${name}?`)) return;
  try {
    await deleteService(name);
    if (selected === name) {
      selected = null;
      text = "";
      els.code.textContent = "";
      els.activePath.textContent = "";
    }
    if (editing === name) resetForm();
    setStatus(`deleted ${name}`, "ok");
    await refresh();
  } catch (err) {
    setStatus(message(err), "error");
  }
}

async function edit(name: string): Promise<void> {
  try {
    const file = await readService(name);
    fillForm(file.spec);
    editing = name;
    els.save.textContent = `Update ${name}`;
    setStatus(`editing ${name}`);
    await select(name);
  } catch (err) {
    setStatus(message(err), "error");
  }
}

async function save(): Promise<void> {
  let spec: ServiceSpec;
  try {
    spec = buildSpec();
  } catch (err) {
    setStatus(message(err), "error");
    return;
  }
  setStatus("saving…");
  try {
    const entry = editing ? await updateService(editing, spec) : await createService(spec);
    setStatus(`stored ${entry.path}`, "ok");
    await refresh();
    await select(entry.name);
  } catch (err) {
    setStatus(message(err), "error");
  }
}

async function refresh(): Promise<void> {
  try {
    entries = await listServices();
    stacks = await listStacks();
    catalogRefs = await catalog();
  } catch (err) {
    setStatus(message(err), "error");
    return;
  }
  renderTree();
  renderStackTemplates();
}

async function showFile(name: string, file: string): Promise<void> {
  try {
    const result = await readServiceFile(name, file);
    text = result.content;
    els.activePath.textContent = `services/${name}/${file}`;
    els.code.innerHTML = hljs.highlight(result.content, { language: "yaml" }).value;
    for (const button of els.files.querySelectorAll<HTMLButtonElement>("button")) {
      button.classList.toggle("active", button.dataset.service === name && button.dataset.file === file);
    }
  } catch (err) {
    setStatus(message(err), "error");
  }
}

function renderTree(): void {
  els.files.replaceChildren();
  if (mode === "service") renderServiceTree();
  else renderStackTree();
}

function renderServiceTree(): void {
  if (entries.length === 0) {
    const empty = document.createElement("div");
    empty.className = "folder";
    empty.textContent = "no services stored";
    els.files.append(empty);
    return;
  }
  for (const entry of entries) {
    const row = document.createElement("div");
    row.className = "file-row";
    const button = document.createElement("button");
    button.className = entry.name === selected ? "file active" : "file";
    button.textContent = entry.name;
    button.title = entry.path;
    button.dataset.name = entry.name;
    const editButton = document.createElement("button");
    editButton.className = "icon";
    editButton.textContent = "✎";
    editButton.title = `Edit ${entry.name}`;
    editButton.dataset.edit = entry.name;
    const deleteButton = document.createElement("button");
    deleteButton.className = "icon";
    deleteButton.textContent = "✕";
    deleteButton.title = `Delete ${entry.name}`;
    deleteButton.dataset.delete = entry.name;
    row.append(button, editButton, deleteButton);
    els.files.append(row);
    for (const file of entry.files ?? []) {
      const leaf = document.createElement("button");
      leaf.className = file === "docker-compose.yml" ? "file leaf main" : "file leaf";
      leaf.textContent = file;
      leaf.title = `services/${entry.name}/${file}`;
      leaf.dataset.file = file;
      leaf.dataset.service = entry.name;
      els.files.append(leaf);
    }
  }
}

async function select(name: string): Promise<void> {
  try {
    const file = await readService(name);
    selected = name;
    text = file.content;
    els.activePath.textContent = file.path;
    els.code.innerHTML = hljs.highlight(file.content, { language: "yaml" }).value;
    for (const button of els.files.querySelectorAll<HTMLButtonElement>("button")) {
      button.classList.toggle("active", button.dataset.name === name && !button.dataset.file);
    }
  } catch (err) {
    setStatus(message(err), "error");
  }
}

async function removeStack(name: string): Promise<void> {
  if (!window.confirm(`Delete the stack ${name}?`)) return;
  try {
    await deleteStack(name);
    if (stackEditing === name) {
      stackEditing = null;
      stackDirect = [];
      stackFeatures = [];
      els.stackName.value = "";
      clearOutput();
    }
    await refresh();
    setStatus(`deleted ${name}`, "ok");
  } catch (err) {
    setStatus(message(err), "error");
  }
}

function imageName(image: string): string {
  let name = image.trim();
  const at = name.indexOf("@");
  if (at >= 0) name = name.slice(0, at);
  if (name.lastIndexOf(":") > name.lastIndexOf("/")) name = name.slice(0, name.lastIndexOf(":"));
  const slash = name.lastIndexOf("/");
  if (slash >= 0) name = name.slice(slash + 1);
  return name;
}

function bind(): void {
  for (const button of document.querySelectorAll<HTMLButtonElement>("button[data-add]")) {
    button.addEventListener("click", () => {
      if (button.dataset.add === "configs") {
        configs.push({ source: "", target: "", content: "" });
        renderConfigs();
      } else if (button.dataset.add === "secrets") {
        secrets.push({ source: "", target: "", type: "file" });
        renderSecrets();
      } else {
        addRow(button);
      }
    });
  }
  els.save.addEventListener("click", () => void save());
  els.files.addEventListener("click", (event) => {
    const target = (event.target as HTMLElement).closest<HTMLButtonElement>("button");
    if (target?.dataset.deleteStack) void removeStack(target.dataset.deleteStack);
    else if (target?.dataset.stack) void showStack(target.dataset.stack);
    else if (target?.dataset.delete) void remove(target.dataset.delete);
    else if (target?.dataset.edit) void edit(target.dataset.edit);
    else if (target?.dataset.member) void showFile(target.dataset.member, "docker-compose.yml");
    else if (target?.dataset.file === "docker-compose.yml" && target.dataset.service) void select(target.dataset.service);
    else if (target?.dataset.file && target.dataset.service) void showFile(target.dataset.service, target.dataset.file);
    else if (target?.dataset.name) void select(target.dataset.name);
  });
  els.new.addEventListener("click", resetForm);
  els.stackSave.addEventListener("click", () => void saveStack());
  els.stackName.addEventListener("input", renderStackPreview);
  els.stackAdd.addEventListener("click", () => {
    const name = els.stackPicker.value;
    if (name === "" || stackDirect.includes(name)) return;
    stackDirect = [...stackDirect, name];
    renderStackTemplates();
  });
  els.stackNew.addEventListener("click", () => {
    stackEditing = null;
    stackDirect = [];
    stackFeatures = [];
    els.stackName.value = "";
    clearOutput();
    renderTree();
    renderStackTemplates();
    renderTreeSelection();
  });
  for (const button of els.modes.querySelectorAll<HTMLButtonElement>("button")) {
    button.addEventListener("click", () => switchMode(button.dataset.mode === "stack" ? "stack" : "service"));
  }
  refreshAdditions = mountAdditions(els.additions, els.addAddition, els.additionEditor, additions, () => undefined);
  els.copy.addEventListener("click", () => {
    if (text === "") return;
    navigator.clipboard.writeText(text).then(
      () => setStatus("copied", "ok"),
      () => setStatus("copy failed", "error"),
    );
  });
  els.download.addEventListener("click", () => {
    if (text === "") return;
    const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = `${selected ?? "service"}.yml`;
    link.click();
    URL.revokeObjectURL(url);
  });
}

renderRows("ports", placeholdersFor("ports"));
renderRows("env", placeholdersFor("env"));
renderRows("labels", placeholdersFor("labels"));
renderRows("buildArgs", placeholdersFor("buildArgs"));
renderRows("buildContexts", placeholdersFor("buildContexts"));
renderRows("buildUlimits", placeholdersFor("buildUlimits"));
renderRows("volumes", placeholdersFor("volumes"));
renderConfigs();
renderSecrets();
bind();
void refresh().then(() => {
  if (entries.length > 0) void select(entries[0].name);
});
