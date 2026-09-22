export interface AuthStatus {
  bootstrap: boolean;
}

export interface AuthSession {
  login: string;
}

export interface APIToken {
  id: string;
  label: string;
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
}

export interface NewAPIToken {
  id: string;
  token: string;
  label: string;
}

export interface Invite {
  token: string;
}

// ApiError — тело {error, field?} из writeAuthError/writeJSON (Go). field
// заполнен только для *auth.ValidationError (422) — остальные коды его не
// несут.
export class ApiError extends Error {
  status: number;
  field?: string;

  constructor(status: number, message: string, field?: string) {
    super(message);
    this.status = status;
    this.field = field;
  }
}

// NO_REFRESH_RETRY_PATHS — эндпоинты, на которых 401 либо не означает
// «истёк access-token» (login/register — это просто неверные креды), либо
// уже сам является попыткой обновления/завершения сессии (refresh/logout) —
// ретраить их через refreshSession() было бы бессмысленно или дало бы
// бесконечную рекурсию.
const NO_REFRESH_RETRY_PATHS = new Set<string>([
  "/api/auth/login",
  "/api/auth/register",
  "/api/auth/refresh",
  "/api/auth/logout",
]);

async function authFetch<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
  const resp = await fetch(path, {
    ...init,
    headers: {
      ...init.headers,
      "Content-Type": "application/json",
      // requireCSRFHeader (этап B, решение 9): обязателен на любом
      // POST/PUT/DELETE под /api/ без исключений, включая /api/auth/*.
      "X-Requested-With": "genodex",
    },
  });

  if (resp.status === 401 && !retried) {
    const bare = path.split("?")[0];
    if (!NO_REFRESH_RETRY_PATHS.has(bare)) {
      // access-cookie истёк (15 минут) — прежде чем считать это отказом,
      // пробуем один раз обновить сессию по refresh-cookie (30 дней,
      // скользящий, auth.md решение 7) и повторить запрос целиком.
      // Безопасно: requireFull на сервере отклоняет запрос ДО разбора
      // тела и бизнес-логики, так что исходная попытка никогда не
      // исполнялась — повтор не может задвоить эффект.
      try {
        await refreshSession();
        return authFetch<T>(path, init, true);
      } catch {
        // refresh тоже не удался — падаем в обычную обработку ниже с
        // исходным 401.
      }
    }
  }

  if (!resp.ok) {
    let body: { error?: string; field?: string } = {};
    try {
      body = await resp.json();
    } catch {
      // тело не JSON (не должно происходить у /api/auth/*, но не валим клиент)
    }
    throw new ApiError(resp.status, body.error ?? `Ошибка ${resp.status}`, body.field);
  }

  if (resp.status === 204) {
    return undefined as T;
  }

  return resp.json();
}

export async function refreshSession(): Promise<AuthSession> {
  return authFetch<AuthSession>("/api/auth/refresh", { method: "POST" });
}

export async function fetchAuthStatus(): Promise<AuthStatus> {
  return authFetch<AuthStatus>("/api/auth/status");
}

// fetchAuthSession: 401 (нет активной сессии) — не ошибка, а ожидаемое
// состояние «анонимный посетитель»; возвращает null вместо throw.
export async function fetchAuthSession(): Promise<AuthSession | null> {
  try {
    return await authFetch<AuthSession>("/api/auth/session");
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      return null;
    }
    throw e;
  }
}

export async function register(
  loginValue: string,
  password: string,
  invite?: string,
): Promise<AuthSession> {
  const qs = invite ? `?invite=${encodeURIComponent(invite)}` : "";
  return authFetch<AuthSession>(`/api/auth/register${qs}`, {
    method: "POST",
    body: JSON.stringify({ login: loginValue, password }),
  });
}

export async function login(loginValue: string, password: string): Promise<AuthSession> {
  return authFetch<AuthSession>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ login: loginValue, password }),
  });
}

export async function logout(): Promise<void> {
  return authFetch<void>("/api/auth/logout", { method: "POST" });
}

// changePassword: успех гасит ВСЕ сессии владельца на сервере (включая
// текущую) — вызывающая сторона (Settings-страница, задача 2) обязана
// после этого отправить пользователя на /login, а не просто обновить
// состояние на месте.
export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  return authFetch<void>("/api/auth/password", {
    method: "POST",
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
}

export async function createInvite(): Promise<Invite> {
  return authFetch<Invite>("/api/auth/invites", { method: "POST" });
}

export async function createAPIToken(label: string): Promise<NewAPIToken> {
  return authFetch<NewAPIToken>("/api/auth/tokens", {
    method: "POST",
    body: JSON.stringify({ label }),
  });
}

export async function listAPITokens(): Promise<APIToken[]> {
  return authFetch<APIToken[]>("/api/auth/tokens");
}

export async function revokeAPIToken(id: string): Promise<void> {
  return authFetch<void>(`/api/auth/tokens/${encodeURIComponent(id)}`, { method: "DELETE" });
}
