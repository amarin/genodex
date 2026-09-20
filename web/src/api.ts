export interface Settlement {
  id: string;
  name: string;
  type: string;
}

export async function fetchSettlements(): Promise<Settlement[]> {
  const resp = await fetch("/api/settlements");
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export interface DocFile {
  path: string;
  title: string;
}

export interface DocListResponse {
  files: DocFile[];
}

export async function fetchDocList(): Promise<DocFile[]> {
  const resp = await fetch("/api/docs");
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  const data: DocListResponse = await resp.json();
  return data.files;
}

export async function fetchDoc(path: string): Promise<string> {
  const resp = await fetch(`/api/docs/${encodeURIComponent(path)}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.text();
}