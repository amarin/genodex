export interface Settlement {
  id: string;
  name: string;
  metadata?: Record<string, string>;
}

export async function fetchSettlements(): Promise<Settlement[]> {
  const resp = await fetch("/api/settlements");
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}