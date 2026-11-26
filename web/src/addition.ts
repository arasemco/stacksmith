import type { Addition, Config, Secret } from "./api";

type Pair = { a: string; b: string };

function control<T extends HTMLElement>(tag: string, value: string, placeholder: string): T {
  const element = document.createElement(tag) as T;
  (element as unknown as { value: string }).value = value;
  element.setAttribute("placeholder", placeholder);
  element.spellcheck = false;
  return element;
}

function kvRows(container: HTMLElement, pairs: Pair[], label: string): void {
  container.replaceChildren();
  pairs.forEach((pair, index) => {
    const row = document.createElement("div");
    row.className = "row-line";
    const key = control<HTMLInputElement>("input", pair.a, "key");
    const value = control<HTMLTextAreaElement>("textarea", pair.b, "value");
    value.rows = 1;
    key.addEventListener("input", () => (pair.a = key.value));
    value.addEventListener("input", () => (pair.b = value.value));
    const remove = document.createElement("button");
    remove.className = "icon";
    remove.textContent = "✕";
    remove.addEventListener("click", () => {
      pairs.splice(index, 1);
      kvRows(container, pairs, label);
    });
    row.append(key, separator(":"), value, remove);
    container.append(row);
  });
}

function separator(symbol: string): HTMLElement {
  const span = document.createElement("span");
  span.className = "sep";
  span.textContent = symbol;
  return span;
}

function labeled(text: string, child: HTMLElement): HTMLElement {
  const wrapper = document.createElement("div");
  wrapper.className = "field";
  const label = document.createElement("label");
  label.textContent = text;
  wrapper.append(label, child);
  return wrapper;
}

function additionsList(container: HTMLElement, additions: Addition[], onEdit: (index: number) => void, onChanged: () => void): void {
  container.replaceChildren();
  additions.forEach((addition, index) => {
    const row = document.createElement("li");
    row.className = "layer";
    const name = document.createElement("span");
    name.className = "name";
    name.textContent = addition.name || "(unnamed)";
    const actions = document.createElement("span");
    actions.className = "row-actions";
    const edit = document.createElement("button");
    edit.className = "icon";
    edit.textContent = "✎";
    edit.addEventListener("click", () => onEdit(index));
    const remove = document.createElement("button");
    remove.className = "icon";
    remove.textContent = "✕";
    remove.addEventListener("click", () => {
      additions.splice(index, 1);
      onChanged();
    });
    actions.append(edit, remove);
    row.append(name, actions);
    container.append(row);
  });
}

export function mountAdditions(
  container: HTMLElement,
  addButton: HTMLButtonElement,
  editor: HTMLElement,
  additions: Addition[],
  onChange: () => void,
): () => void {
  const render = (): void => {
    additionsList(
      container,
      additions,
      (index) => open(additions[index], index),
      () => {
        render();
        onChange();
      },
    );
  };

  const close = (): void => {
    editor.hidden = true;
    editor.replaceChildren();
  };

  function open(addition: Addition, index: number): void {
    const working: Addition = structuredClone(addition);
    const labels = Object.entries(working.labels ?? {}).map(([key, value]) => ({ a: key, b: value }));
    const environment = Object.entries(working.environment ?? {}).map(([key, value]) => ({ a: key, b: value }));
    const volumes = (working.volumes ?? []).map((item) => {
      const at = item.indexOf(":");
      return at < 0 ? { a: "", b: item } : { a: item.slice(0, at), b: item.slice(at + 1) };
    });

    const configs = working.configs ?? [];
    const secrets = working.secrets ?? [];

    editor.replaceChildren();
    editor.hidden = false;

    const name = control<HTMLInputElement>("input", working.name ?? "", "database/mariadb");
    editor.append(labeled("name *", name));

    const dependBox = document.createElement("label");
    dependBox.className = "check";
    const dependInput = document.createElement("input");
    dependInput.type = "checkbox";
    dependInput.checked = working.depend !== false;
    dependBox.append(dependInput, document.createTextNode(" depend on this service"));
    editor.append(dependBox);

    const labelsBox = document.createElement("div");
    const labelsAdd = document.createElement("button");
    labelsAdd.className = "add-row";
    labelsAdd.textContent = "+ Add label";
    labelsAdd.addEventListener("click", () => {
      labels.push({ a: "", b: "" });
      kvRows(labelsBox, labels, "label");
    });
    kvRows(labelsBox, labels, "label");
    editor.append(labeled("labels", labelsBox), labelsAdd);

    const envBox = document.createElement("div");
    const envAdd = document.createElement("button");
    envAdd.className = "add-row";
    envAdd.textContent = "+ Add env var";
    envAdd.addEventListener("click", () => {
      environment.push({ a: "", b: "" });
      kvRows(envBox, environment, "env");
    });
    kvRows(envBox, environment, "env");
    editor.append(labeled("environment", envBox), envAdd);

    const volumesBox = document.createElement("div");
    const volumesAdd = document.createElement("button");
    volumesAdd.className = "add-row";
    volumesAdd.textContent = "+ Add volume";
    volumesAdd.addEventListener("click", () => {
      volumes.push({ a: "", b: "" });
      kvRows(volumesBox, volumes, "volume");
    });
    kvRows(volumesBox, volumes, "volume");
    editor.append(labeled("volumes", volumesBox), volumesAdd);

    const configsBox = document.createElement("div");
    configsBox.className = "rows";
    const renderConfigs = (): void => {
      configsBox.replaceChildren();
      configs.forEach((config, index) => {
        const row = document.createElement("div");
        row.className = "config-row";
        const head = document.createElement("div");
        head.className = "config-head";
        const source = control<HTMLInputElement>("input", config.source ?? "", "source");
        source.addEventListener("input", () => (config.source = source.value));
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
        const target = control<HTMLInputElement>("input", config.target ?? "", "/container/path");
        target.addEventListener("input", () => (config.target = target.value));
        const remove = document.createElement("button");
        remove.className = "icon";
        remove.textContent = "✕";
        remove.addEventListener("click", () => {
          configs.splice(index, 1);
          renderConfigs();
        });
        head.append(source, type, target, remove);
        row.append(head);
        if ((config.type ?? "content") === "content") {
          const content = control<HTMLTextAreaElement>("textarea", config.content ?? "", "content");
          content.rows = 3;
          content.addEventListener("input", () => (config.content = content.value));
          row.append(content);
        } else if (config.type === "file") {
          const file = control<HTMLInputElement>("input", config.file ?? "", "./config.conf");
          file.addEventListener("input", () => (config.file = file.value));
          row.append(file);
        }
        configsBox.append(row);
      });
    };
    renderConfigs();
    const configsAdd = document.createElement("button");
    configsAdd.className = "add-row";
    configsAdd.textContent = "+ Add config";
    configsAdd.addEventListener("click", () => {
      configs.push({ source: "", type: "content" });
      renderConfigs();
    });
    editor.append(labeled("configs", configsBox), configsAdd);

    const secretsBox = document.createElement("div");
    secretsBox.className = "rows";
    const renderSecrets = (): void => {
      secretsBox.replaceChildren();
      secrets.forEach((secret, index) => {
        const row = document.createElement("div");
        row.className = "config-row";
        const head = document.createElement("div");
        head.className = "config-head";
        const source = control<HTMLInputElement>("input", secret.source ?? "", "source");
        source.addEventListener("input", () => (secret.source = source.value));
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
        const target = control<HTMLInputElement>("input", secret.target ?? "", "/run/secrets/name");
        target.addEventListener("input", () => (secret.target = target.value));
        const remove = document.createElement("button");
        remove.className = "icon";
        remove.textContent = "✕";
        remove.addEventListener("click", () => {
          secrets.splice(index, 1);
          renderSecrets();
        });
        head.append(source, type, target, remove);
        row.append(head);
        if (secret.type === "environment") {
          const variable = control<HTMLInputElement>("input", secret.environment ?? "", "ENV_VAR");
          variable.addEventListener("input", () => (secret.environment = variable.value));
          row.append(variable);
        } else if ((secret.type ?? "file") === "file") {
          const file = control<HTMLInputElement>("input", secret.file ?? "", "./secret.txt");
          file.addEventListener("input", () => (secret.file = file.value));
          row.append(file);
        }
        secretsBox.append(row);
      });
    };
    renderSecrets();
    const secretsAdd = document.createElement("button");
    secretsAdd.className = "add-row";
    secretsAdd.textContent = "+ Add secret";
    secretsAdd.addEventListener("click", () => {
      secrets.push({ source: "", type: "file" });
      renderSecrets();
    });
    editor.append(labeled("secrets", secretsBox), secretsAdd);

    const save = document.createElement("button");
    save.className = "primary save";
    save.textContent = "Save addition";
    const cancel = document.createElement("button");
    cancel.textContent = "Cancel";
    cancel.addEventListener("click", close);

    save.addEventListener("click", () => {
      const next: Addition = {
        name: name.value.trim(),
        depend: dependInput.checked,
        ...(labels.some((pair) => pair.a.trim())
          ? {
              labels: Object.fromEntries(
                labels.filter((pair) => pair.a.trim()).map((pair) => [pair.a.trim(), pair.b]),
              ),
            }
          : {}),
        ...(environment.some((pair) => pair.a.trim())
          ? {
              environment: Object.fromEntries(
                environment.filter((pair) => pair.a.trim()).map((pair) => [pair.a.trim(), pair.b]),
              ),
            }
          : {}),
        ...(volumes.some((item) => item.b.trim())
          ? {
              volumes: volumes
                .filter((item) => item.b.trim())
                .map((item) => (item.a.trim() ? `${item.a.trim()}:${item.b.trim()}` : item.b.trim())),
            }
          : {}),
        ...(configs.some((config) => config.source.trim()) ? { configs } : {}),
        ...(secrets.some((secret) => secret.source.trim()) ? { secrets } : {}),
      };
      if (index >= 0) additions[index] = next;
      else additions.push(next);
      close();
      render();
      onChange();
    });

    const actions = document.createElement("div");
    actions.className = "form-actions";
    actions.append(cancel, save);
    editor.append(actions);
  }

  addButton.addEventListener("click", () => open({ name: "" }, -1));
  render();
  return render;
}
