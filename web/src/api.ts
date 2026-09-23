import { authFetch } from "./auth";

export interface AdminDivision {
  id: string;
  name: string;
  type: string;
  parent_id: string | null;
  sources: SourceLink[];
}

// AdminDivisionType — models.AdminDivisionType (internal/models/administrative_division_type.go).
export type AdminDivisionType =
  | "governorate"
  | "district"
  | "volost"
  | "other"
  | "gorod"
  | "selo"
  | "derevnya"
  | "hutor"
  | "pogost"
  | "stanitsa"
  | "mestechko";

export const ADMIN_DIVISION_TYPE_LABELS: Record<AdminDivisionType, string> = {
  governorate: "Губерния",
  district: "Уезд",
  volost: "Волость",
  other: "Иное",
  gorod: "Город",
  selo: "Село",
  derevnya: "Деревня",
  hutor: "Хутор",
  pogost: "Погост",
  stanitsa: "Станица",
  mestechko: "Местечко",
};

export function adminDivisionTypeLabel(t: string): string {
  return ADMIN_DIVISION_TYPE_LABELS[t as AdminDivisionType] ?? t;
}

// AdminDivisionInput — тело POST/PUT /api/admin-divisions (transport.AdminDivisionCreate
// и transport.AdminDivisionUpdate имеют одинаковую форму: полная замена name/type/parent_id;
// с подпроекта 5 сюда же входит sources — тоже полная замена).
export interface AdminDivisionInput {
  name: string;
  type: AdminDivisionType;
  parent_id: string | null;
  sources: SourceLink[];
}

export interface AdminDivisionQuery {
  kind?: "settlement";
  type?: string;
  parent_id?: string;
  limit?: number;
  offset?: number;
}

export interface DivisionSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

// Максимальный размер окна списка (совпадает с MaxPageLimit на сервере).
export const MAX_PAGE_LIMIT = 500;

export async function fetchAdminDivisions(
  query: AdminDivisionQuery = {},
): Promise<AdminDivision[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/admin-divisions${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchAdminDivisions(
  query: DivisionSearchQuery,
): Promise<AdminDivision[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/admin-divisions/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchDivision — GET /api/admin-divisions/{id}, открыто анонимному
// посетителю (auth.md §6). Через authFetch — ради ApiError (нужен код 404
// на странице View: единицу могли удалить в другой вкладке).
export async function fetchDivision(id: string): Promise<AdminDivision> {
  return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`);
}

// createDivision/updateDivision/deleteDivision — запись, только для
// вошедшего владельца (requireFull на сервере, internal/httpapi/division_write.go).
export async function createDivision(input: AdminDivisionInput): Promise<AdminDivision> {
  return authFetch<AdminDivision>("/api/admin-divisions", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateDivision(id: string, input: AdminDivisionInput): Promise<AdminDivision> {
  return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteDivision(id: string): Promise<void> {
  return authFetch<void>(`/api/admin-divisions/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// TextRef — контракт элемента списков вроде Surname.variants: текст или
// ссылка на другую сущность (transport.TextRef). v1-формы редактируют
// только text; ref/type только читаются (docs/data-model/entity-write.md §4).
export interface TextRef {
  text: string;
  ref?: string;
  type?: string;
}

export interface Surname {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// SurnameInput — тело POST/PUT /api/surnames (transport.SurnameCreate и
// transport.SurnameUpdate имеют одинаковую форму: полная замена всех полей).
export interface SurnameInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface SurnameQuery {
  limit?: number;
  offset?: number;
}

export interface SurnameSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchSurnames(query: SurnameQuery = {}): Promise<Surname[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/surnames${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchSurnames(query: SurnameSearchQuery): Promise<Surname[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/surnames/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchSurname — GET /api/surnames/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchSurname(id: string): Promise<Surname> {
  return authFetch<Surname>(`/api/surnames/${encodeURIComponent(id)}`);
}

// createSurname/updateSurname/deleteSurname — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/surname_write.go).
export async function createSurname(input: SurnameInput): Promise<Surname> {
  return authFetch<Surname>("/api/surnames", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateSurname(id: string, input: SurnameInput): Promise<Surname> {
  return authFetch<Surname>(`/api/surnames/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteSurname(id: string): Promise<void> {
  return authFetch<void>(`/api/surnames/${encodeURIComponent(id)}`, { method: "DELETE" });
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

export interface Patronymic {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// PatronymicInput — тело POST/PUT /api/patronymics (transport.PatronymicCreate и
// transport.PatronymicUpdate имеют одинаковую форму: полная замена всех полей).
export interface PatronymicInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface PatronymicQuery {
  limit?: number;
  offset?: number;
}

export interface PatronymicSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchPatronymics(query: PatronymicQuery = {}): Promise<Patronymic[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/patronymics${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchPatronymics(query: PatronymicSearchQuery): Promise<Patronymic[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/patronymics/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchPatronymic — GET /api/patronymics/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchPatronymic(id: string): Promise<Patronymic> {
  return authFetch<Patronymic>(`/api/patronymics/${encodeURIComponent(id)}`);
}

// createPatronymic/updatePatronymic/deletePatronymic — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/patronymic_write.go).
export async function createPatronymic(input: PatronymicInput): Promise<Patronymic> {
  return authFetch<Patronymic>("/api/patronymics", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updatePatronymic(id: string, input: PatronymicInput): Promise<Patronymic> {
  return authFetch<Patronymic>(`/api/patronymics/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deletePatronymic(id: string): Promise<void> {
  return authFetch<void>(`/api/patronymics/${encodeURIComponent(id)}`, { method: "DELETE" });
}


export interface Estate {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// EstateInput — тело POST/PUT /api/estates (transport.EstateCreate и
// transport.EstateUpdate имеют одинаковую форму: полная замена всех полей).
export interface EstateInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface EstateQuery {
  limit?: number;
  offset?: number;
}

export interface EstateSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchEstates(query: EstateQuery = {}): Promise<Estate[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/estates${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchEstates(query: EstateSearchQuery): Promise<Estate[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/estates/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchEstate — GET /api/estates/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchEstate(id: string): Promise<Estate> {
  return authFetch<Estate>(`/api/estates/${encodeURIComponent(id)}`);
}

// createEstate/updateEstate/deleteEstate — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/estate_write.go).
export async function createEstate(input: EstateInput): Promise<Estate> {
  return authFetch<Estate>("/api/estates", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateEstate(id: string, input: EstateInput): Promise<Estate> {
  return authFetch<Estate>(`/api/estates/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteEstate(id: string): Promise<void> {
  return authFetch<void>(`/api/estates/${encodeURIComponent(id)}`, { method: "DELETE" });
}


export interface Title {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// TitleInput — тело POST/PUT /api/titles (transport.TitleCreate и
// transport.TitleUpdate имеют одинаковую форму: полная замена всех полей).
export interface TitleInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface TitleQuery {
  limit?: number;
  offset?: number;
}

export interface TitleSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchTitles(query: TitleQuery = {}): Promise<Title[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/titles${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchTitles(query: TitleSearchQuery): Promise<Title[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/titles/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchTitle — GET /api/titles/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchTitle(id: string): Promise<Title> {
  return authFetch<Title>(`/api/titles/${encodeURIComponent(id)}`);
}

// createTitle/updateTitle/deleteTitle — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/title_write.go).
export async function createTitle(input: TitleInput): Promise<Title> {
  return authFetch<Title>("/api/titles", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateTitle(id: string, input: TitleInput): Promise<Title> {
  return authFetch<Title>(`/api/titles/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteTitle(id: string): Promise<void> {
  return authFetch<void>(`/api/titles/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export type NameGender = "male" | "female" | "neutral";

export interface GivenName {
  id: string;
  canonical: string;
  gender: NameGender | "";
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// GivenNameInput — тело POST/PUT /api/given-names (transport.GivenNameCreate и
// transport.GivenNameUpdate имеют одинаковую форму: полная замена всех полей).
export interface GivenNameInput {
  canonical: string;
  gender: NameGender;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface GivenNameQuery {
  limit?: number;
  offset?: number;
}

export interface GivenNameSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchGivenNames(query: GivenNameQuery = {}): Promise<GivenName[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/given-names${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchGivenNames(query: GivenNameSearchQuery): Promise<GivenName[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/given-names/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchGivenName — GET /api/given-names/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchGivenName(id: string): Promise<GivenName> {
  return authFetch<GivenName>(`/api/given-names/${encodeURIComponent(id)}`);
}

// createGivenName/updateGivenName/deleteGivenName — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/given_name_write.go).
export async function createGivenName(input: GivenNameInput): Promise<GivenName> {
  return authFetch<GivenName>("/api/given-names", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateGivenName(id: string, input: GivenNameInput): Promise<GivenName> {
  return authFetch<GivenName>(`/api/given-names/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteGivenName(id: string): Promise<void> {
  return authFetch<void>(`/api/given-names/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface SourceLink {
  citation_id: string;
  target_type?: string;
  target_id?: string;
  reliability?: string;
  role?: string;
  note?: string;
}

// FactDate — контракт структурированной даты (transport.FactDate): год/
// месяц/день, точность, формулировка, календарь, верхняя граница периода для
// modifier=between. Первое появление в проекте (Parish.Since/Until).
export type FactPrecision = "unknown" | "year" | "month" | "day";
export type FactModifier = "exact" | "approx" | "before" | "after" | "between";
export type FactCalendar = "" | "gregorian" | "julian" | "unknown";

export interface FactDate {
  year: number;
  month?: number;
  day?: number;
  precision: FactPrecision;
  modifier: FactModifier;
  calendar?: FactCalendar;
  year_to?: number;
  month_to?: number;
  day_to?: number;
}

export interface Repository {
  id: string;
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// RepositoryInput — тело POST/PUT /api/repositories (transport.RepositoryCreate
// и transport.RepositoryUpdate имеют одинаковую форму: полная замена всех
// полей, включая sources — редактируется с подпроекта 5).
export interface RepositoryInput {
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface RepositoryQuery {
  limit?: number;
  offset?: number;
}

export interface RepositorySearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchRepositories(query: RepositoryQuery = {}): Promise<Repository[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/repositories${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchRepositories(query: RepositorySearchQuery): Promise<Repository[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/repositories/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchRepository(id: string): Promise<Repository> {
  return authFetch<Repository>(`/api/repositories/${encodeURIComponent(id)}`);
}

export async function createRepository(input: RepositoryInput): Promise<Repository> {
  return authFetch<Repository>("/api/repositories", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateRepository(id: string, input: RepositoryInput): Promise<Repository> {
  return authFetch<Repository>(`/api/repositories/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteRepository(id: string): Promise<void> {
  return authFetch<void>(`/api/repositories/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Family {
  id: string;
  name: string;
  members: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// FamilyInput — тело POST/PUT /api/families (transport.FamilyCreate и
// transport.FamilyUpdate имеют одинаковую форму: полная замена всех полей,
// включая sources — редактируется с рождения контракта, docs/data-model/
// entity-write.md §3.3/§3.6).
export interface FamilyInput {
  name: string;
  members: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface FamilyQuery {
  limit?: number;
  offset?: number;
}

export interface FamilySearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchFamilies(query: FamilyQuery = {}): Promise<Family[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/families${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchFamilies(query: FamilySearchQuery): Promise<Family[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/families/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchFamily(id: string): Promise<Family> {
  return authFetch<Family>(`/api/families/${encodeURIComponent(id)}`);
}

export async function createFamily(input: FamilyInput): Promise<Family> {
  return authFetch<Family>("/api/families", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateFamily(id: string, input: FamilyInput): Promise<Family> {
  return authFetch<Family>(`/api/families/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteFamily(id: string): Promise<void> {
  return authFetch<void>(`/api/families/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Church {
  id: string;
  name: string;
  parish?: TextRef | null;
  settlements: TextRef[];
  variants: string[];
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ChurchInput {
  name: string;
  parish?: TextRef | null;
  settlements: TextRef[];
  variants: string[];
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ChurchQuery {
  limit?: number;
  offset?: number;
}

export interface ChurchSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchChurches(query: ChurchQuery = {}): Promise<Church[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/churches${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchChurches(query: ChurchSearchQuery): Promise<Church[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/churches/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchChurch(id: string): Promise<Church> {
  return authFetch<Church>(`/api/churches/${encodeURIComponent(id)}`);
}

export async function createChurch(input: ChurchInput): Promise<Church> {
  return authFetch<Church>("/api/churches", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateChurch(id: string, input: ChurchInput): Promise<Church> {
  return authFetch<Church>(`/api/churches/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteChurch(id: string): Promise<void> {
  return authFetch<void>(`/api/churches/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Parish {
  id: string;
  name: string;
  church?: TextRef | null;
  settlements: TextRef[];
  since?: FactDate | null;
  until?: FactDate | null;
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ParishInput {
  name: string;
  church?: TextRef | null;
  settlements: TextRef[];
  since?: FactDate | null;
  until?: FactDate | null;
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ParishQuery {
  limit?: number;
  offset?: number;
}

export interface ParishSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchParishes(query: ParishQuery = {}): Promise<Parish[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/parishes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchParishes(query: ParishSearchQuery): Promise<Parish[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/parishes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchParish(id: string): Promise<Parish> {
  return authFetch<Parish>(`/api/parishes/${encodeURIComponent(id)}`);
}

export async function createParish(input: ParishInput): Promise<Parish> {
  return authFetch<Parish>("/api/parishes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateParish(id: string, input: ParishInput): Promise<Parish> {
  return authFetch<Parish>(`/api/parishes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteParish(id: string): Promise<void> {
  return authFetch<void>(`/api/parishes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Archive {
  id: string;
  name: string;
  system?: TextRef | null;
  repository_id?: string;
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// ArchiveInput — repository_id — просто id (не TextRef, в отличие от
// system/parish/church у других сущностей): пустая строка — без хранилища.
export interface ArchiveInput {
  name: string;
  system?: TextRef | null;
  repository_id?: string;
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveQuery {
  limit?: number;
  offset?: number;
}

export interface ArchiveSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchives(query: ArchiveQuery = {}): Promise<Archive[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archives${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchArchives(query: ArchiveSearchQuery): Promise<Archive[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archives/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchive(id: string): Promise<Archive> {
  return authFetch<Archive>(`/api/archives/${encodeURIComponent(id)}`);
}

export async function createArchive(input: ArchiveInput): Promise<Archive> {
  return authFetch<Archive>("/api/archives", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchive(id: string, input: ArchiveInput): Promise<Archive> {
  return authFetch<Archive>(`/api/archives/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchive(id: string): Promise<void> {
  return authFetch<void>(`/api/archives/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Note — заметка (markdown-текст с иерархией «книга → главы»). parent_id —
// просто id родительской заметки (не TextRef — строгая self-ref ссылка, как
// у Archive.repository_id); пустая строка — без родителя. sources
// редактируется с подпроекта 5 (Citation теперь имеет CRUD).
export interface Note {
  id: string;
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  sources: SourceLink[];
  private: boolean;
}

export interface NoteInput {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  sources: SourceLink[];
  private: boolean;
}

export interface NoteQuery {
  limit?: number;
  offset?: number;
}

export interface NoteSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchNotes(query: NoteQuery = {}): Promise<Note[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/notes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchNotes(query: NoteSearchQuery): Promise<Note[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/notes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchNote(id: string): Promise<Note> {
  return authFetch<Note>(`/api/notes/${encodeURIComponent(id)}`);
}

export async function createNote(input: NoteInput): Promise<Note> {
  return authFetch<Note>("/api/notes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateNote(id: string, input: NoteInput): Promise<Note> {
  return authFetch<Note>(`/api/notes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteNote(id: string): Promise<void> {
  return authFetch<void>(`/api/notes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Attachment — файловое вложение. node_id — обязательная строгая ссылка на
// архивный узел (просто id — строка, не объект). document_id —
// необязательная мягкая ссылка (ON DELETE SET NULL). Оба поля в веб-форме
// заполняются через ArchiveNodePicker/ArchiveDocumentSelect (см.
// ArchiveNodePicker.tsx).
export interface Attachment {
  id: string;
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private: boolean;
}

export interface AttachmentInput {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private: boolean;
}

export interface AttachmentQuery {
  limit?: number;
  offset?: number;
}

export interface AttachmentSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchAttachments(query: AttachmentQuery = {}): Promise<Attachment[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/attachments${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchAttachments(query: AttachmentSearchQuery): Promise<Attachment[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/attachments/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchAttachment(id: string): Promise<Attachment> {
  return authFetch<Attachment>(`/api/attachments/${encodeURIComponent(id)}`);
}

export async function createAttachment(input: AttachmentInput): Promise<Attachment> {
  return authFetch<Attachment>("/api/attachments", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateAttachment(id: string, input: AttachmentInput): Promise<Attachment> {
  return authFetch<Attachment>(`/api/attachments/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteAttachment(id: string): Promise<void> {
  return authFetch<void>(`/api/attachments/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export type Reliability = "primary" | "contemporary" | "memory" | "indirect" | "unknown";

// Anchor — контракт полиморфной привязки «где именно» (transport.Anchor):
// плоское представление с дискриминатором kind, по образцу FactDate. undefined
// — привязки нет (цитата может относиться к источнику целиком).
export type AnchorKind = "archive" | "file" | "url";

export interface Anchor {
  kind: AnchorKind;
  node_id?: string;
  document_id?: string;
  page?: number;
  rect?: string;
  attachment_id?: string;
  timecode?: string;
  url?: string;
}

// Source — источник доказательства. date — структурированная дата (см.
// FactDate). repository_id — просто id (мягкая ссылка, необязательна).
export interface Source {
  id: string;
  kind: string;
  title: string;
  author?: string;
  date?: FactDate | null;
  reliability: Reliability;
  repository_id?: string;
  notes: TextRef[];
  private: boolean;
}

export interface SourceInput {
  kind: string;
  title: string;
  author?: string;
  date?: FactDate | null;
  reliability: Reliability;
  repository_id?: string;
  notes: TextRef[];
  private: boolean;
}

export interface SourceQuery {
  limit?: number;
  offset?: number;
}

export interface SourceSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchSources(query: SourceQuery = {}): Promise<Source[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/sources${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchSources(query: SourceSearchQuery): Promise<Source[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/sources/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchSource(id: string): Promise<Source> {
  return authFetch<Source>(`/api/sources/${encodeURIComponent(id)}`);
}

export async function createSource(input: SourceInput): Promise<Source> {
  return authFetch<Source>("/api/sources", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateSource(id: string, input: SourceInput): Promise<Source> {
  return authFetch<Source>(`/api/sources/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteSource(id: string): Promise<void> {
  return authFetch<void>(`/api/sources/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Citation — цитата из источника. anchor — необязательная полиморфная
// привязка «где именно» (см. Anchor).
export interface Citation {
  id: string;
  source_id: string;
  anchor?: Anchor | null;
  text?: string;
  note?: string;
  private: boolean;
}

export interface CitationInput {
  source_id: string;
  anchor?: Anchor | null;
  text?: string;
  note?: string;
  private: boolean;
}

export interface CitationQuery {
  limit?: number;
  offset?: number;
}

export interface CitationSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchCitations(query: CitationQuery = {}): Promise<Citation[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/citations${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchCitations(query: CitationSearchQuery): Promise<Citation[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/citations/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchCitation(id: string): Promise<Citation> {
  return authFetch<Citation>(`/api/citations/${encodeURIComponent(id)}`);
}

export async function createCitation(input: CitationInput): Promise<Citation> {
  return authFetch<Citation>("/api/citations", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateCitation(id: string, input: CitationInput): Promise<Citation> {
  return authFetch<Citation>(`/api/citations/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteCitation(id: string): Promise<void> {
  return authFetch<void>(`/api/citations/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// ArchiveNode — узел архивного дерева (transport.ArchiveNode). Дерево
// скопировано по обязательному ArchiveID — не одно глобальное дерево, как
// AdminDivision, а по одному на архив (docs/data-model/entity-write.md §3.4).
// ParentID, если задан, — родитель В ПРЕДЕЛАХ ТОГО ЖЕ архива (сервер это
// проверяет — 422 на parent_id, если родитель из другого архива).
export interface ArchiveNode {
  id: string;
  type: string;
  archive_id: string;
  parent_id?: string | null;
  label: string;
  name: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveNodeInput {
  type: string;
  archive_id: string;
  parent_id?: string | null;
  label: string;
  name: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// ArchiveNodeQuery — archive_id ОБЯЗАТЕЛЕН (сервер отдаёт 400 без него, узел
// бессмысленен вне архива). parent_id — как у AdminDivisionQuery: не задан —
// корень (внутри архива), иначе — прямые дети.
export interface ArchiveNodeQuery {
  archive_id: string;
  parent_id?: string;
  limit?: number;
  offset?: number;
}

export interface ArchiveNodeSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchiveNodes(query: ArchiveNodeQuery): Promise<ArchiveNode[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-nodes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// searchArchiveNodes — GET /api/archive-nodes/search?q=. Глобальный поиск, НЕ
// ограничен archive_id (в отличие от fetchArchiveNodes) — сужение по архиву,
// если нужно, делает вызывающая сторона на клиенте (см. ArchiveNodesList.tsx).
export async function searchArchiveNodes(query: ArchiveNodeSearchQuery): Promise<ArchiveNode[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-nodes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchiveNode(id: string): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>(`/api/archive-nodes/${encodeURIComponent(id)}`);
}

export async function createArchiveNode(input: ArchiveNodeInput): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>("/api/archive-nodes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchiveNode(id: string, input: ArchiveNodeInput): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>(`/api/archive-nodes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchiveNode(id: string): Promise<void> {
  return authFetch<void>(`/api/archive-nodes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// ArchiveDocument — документ внутри единицы учёта (transport.ArchiveDocument).
// В отличие от ArchiveNode — плоская сущность, без собственной иерархии;
// unit_id — обязательная строгая ссылка на ArchiveNode.
export interface ArchiveDocument {
  id: string;
  unit_id: string;
  title: string;
  kind: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveDocumentInput {
  unit_id: string;
  title: string;
  kind: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveDocumentQuery {
  limit?: number;
  offset?: number;
}

export interface ArchiveDocumentSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchiveDocuments(
  query: ArchiveDocumentQuery = {},
): Promise<ArchiveDocument[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-documents${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchArchiveDocuments(
  query: ArchiveDocumentSearchQuery,
): Promise<ArchiveDocument[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-documents/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchiveDocument(id: string): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>(`/api/archive-documents/${encodeURIComponent(id)}`);
}

export async function createArchiveDocument(input: ArchiveDocumentInput): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>("/api/archive-documents", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchiveDocument(
  id: string,
  input: ArchiveDocumentInput,
): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>(`/api/archive-documents/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchiveDocument(id: string): Promise<void> {
  return authFetch<void>(`/api/archive-documents/${encodeURIComponent(id)}`, { method: "DELETE" });
}