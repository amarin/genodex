import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { fetchAuthStatus, fetchAuthSession, type AuthSession } from "./auth";

// --- сессия: контекст, доступный шапке и страницам Login/Register/Settings ---

interface SessionState {
  loading: boolean;
  bootstrap: boolean;
  session: AuthSession | null; // null — анонимный посетитель
}

interface SessionContextValue extends SessionState {
  refresh: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | null>(null);

// useSession — текущая сессия и способ её обновить (после
// login/register/logout/смены пароля). Бросает, если вызван вне дерева
// SessionProvider — единственное дерево строит App() ниже, так что в
// пределах этого приложения это всегда программная ошибка, не рантайм-кейс.
export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (ctx == null) {
    throw new Error("useSession вызван вне SessionProvider");
  }
  return ctx;
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<SessionState>({ loading: true, bootstrap: false, session: null });
  const navigate = useNavigate();
  const location = useLocation();

  const refresh = useCallback(async () => {
    const status = await fetchAuthStatus();
    const session = status.bootstrap ? null : await fetchAuthSession();
    setState({ loading: false, bootstrap: status.bootstrap, session });
  }, []);

  useEffect(() => {
    refresh().catch(() => setState((s) => ({ ...s, loading: false })));
  }, [refresh]);

  // Корень приложения на старте (auth.md §7): bootstrap=true — редирект на
  // /register. Проверяется не только на первом рендере — пока bootstrap
  // остаётся true, любая другая страница тоже отправляет туда же.
  useEffect(() => {
    if (!state.loading && state.bootstrap && location.pathname !== "/register") {
      navigate("/register", { replace: true });
    }
  }, [state.loading, state.bootstrap, location.pathname, navigate]);

  return <SessionContext.Provider value={{ ...state, refresh }}>{children}</SessionContext.Provider>;
}
