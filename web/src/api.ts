export type Config = {
  source: string;
  type?: "content" | "file" | "external";
  target?: string;
  content?: string;
  file?: string;
};

export type Secret = {
  source: string;
  type?: "file" | "environment" | "external";
  target?: string;
  file?: string;
  environment?: string;
};

export type Addition = {
  name: string;
  depend?: boolean;
  labels?: Record<string, string>;
  environment?: Record<string, string>;
  volumes?: string[];
  configs?: Config[];
  secrets?: Secret[];
};

export type Build = {
  context?: string;
  dockerfile?: string;
  dockerfile_inline?: string;
  target?: string;
  args?: Record<string, string>;
  cache_from?: string[];
  cache_to?: string[];
  platforms?: string[];
  tags?: string[];
  network?: string;
  shm_size?: string;
  pull?: boolean;
  no_cache?: boolean;
  ssh?: string[];
  additional_contexts?: Record<string, string>;
  extra_hosts?: string[];
  entitlements?: string[];
  isolation?: string;
  ulimits?: Record<string, number>;
};

export type Deploy = {
  cpus?: string;
  memory?: string;
  replicas?: number;
};

export type Healthcheck = {
  test: string;
  interval?: string;
  timeout?: string;
  retries?: number;
  start_period?: string;
};

export type ServiceSpec = {
  name: string;
  image?: string;
  build?: Build;
  restart?: string;
  pull_policy?: string;
  command?: string;
  ports?: string[];
  environment?: Record<string, string>;
  volumes?: string[];
  healthcheck?: Healthcheck;
  configs?: Config[];
  secrets?: Secret[];
  profiles?: string[];
  labels?: Record<string, string>;
  deploy?: Deploy;
  additions?: Addition[];
};

export type ServiceRef = { name: string; additions?: Addition[] };

export type StackSpec = { name: string; templates: string[] };

export type StoredStack = { name: string; path: string; content: string; spec: StackSpec };

export type Entry = { name: string; path: string; files?: string[] };
export type StoredService = { name: string; path: string; content: string; spec: ServiceSpec };

async function json<T>(res: Response): Promise<T> {
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
  return (await res.json()) as T;
}

export async function listServices(): Promise<Entry[]> {
  const res = await fetch("v1/services");
  const body = await json<{ services: Entry[] }>(res);
  return body.services ?? [];
}

export async function createService(spec: ServiceSpec): Promise<Entry> {
  const res = await fetch("v1/services", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(spec),
  });
  return json<Entry>(res);
}

export async function readServiceFile(name: string, path: string): Promise<{ path: string; content: string }> {
  const res = await fetch(`v1/services/${encodeURIComponent(name)}/files/${path}`);
  return json<{ path: string; content: string }>(res);
}

export async function readService(name: string): Promise<StoredService> {
  const res = await fetch(`v1/services/${encodeURIComponent(name)}`);
  return json<StoredService>(res);
}

export async function updateService(name: string, spec: ServiceSpec): Promise<Entry> {
  const res = await fetch(`v1/services/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(spec),
  });
  return json<Entry>(res);
}

export async function deleteService(name: string): Promise<void> {
  const res = await fetch(`v1/services/${encodeURIComponent(name)}`, { method: "DELETE" });
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
}

export async function listStacks(): Promise<Entry[]> {
  const res = await fetch("v1/stacks");
  const body = await json<{ stacks: Entry[] }>(res);
  return body.stacks;
}

export async function createStack(spec: StackSpec): Promise<Entry> {
  const res = await fetch("v1/stacks", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(spec),
  });
  return json<Entry>(res);
}

export async function readStack(name: string): Promise<StoredStack> {
  const res = await fetch(`v1/stacks/${encodeURIComponent(name)}`);
  return json<StoredStack>(res);
}

export async function updateStack(name: string, spec: StackSpec): Promise<Entry> {
  const res = await fetch(`v1/stacks/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(spec),
  });
  return json<Entry>(res);
}

export async function deleteStack(name: string): Promise<void> {
  const res = await fetch(`v1/stacks/${encodeURIComponent(name)}`, { method: "DELETE" });
  if (!res.ok) throw new Error(await res.text());
}

export async function catalog(): Promise<ServiceRef[]> {
  const res = await fetch("v1/catalog");
  const body = await json<{ catalog: ServiceRef[] }>(res);
  return body.catalog;
}
