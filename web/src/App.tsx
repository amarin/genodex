import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  Link,
  useLocation,
  useNavigate,
} from "react-router-dom";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { Alert, Button, Card, Input, Layout, List, Space, Spin, Tabs, Typography } from "antd";
import { BookOutlined, HomeOutlined } from "@ant-design/icons";
import {
  fetchAdminDivisions,
  searchAdminDivisions,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type AdminDivisionQuery,
} from "./api";
import { fetchAuthStatus, fetchAuthSession, logout, type AuthSession } from "./auth";
import DocsPanel from "./docs-panel";
import LoginPage from "./pages/Login";
import RegisterPage from "./pages/Register";
import SettingsPage from "./pages/Settings";

const { Header, Content } = Layout;

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

function SessionProvider({ children }: { children: ReactNode }) {
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

// --- общая шапка: используется и /docs (ниже), и страницами Login/Register/Settings ---

export function AppHeader() {
  const { loading, session, refresh } = useSession();
  const navigate = useNavigate();

  const onLogout = async () => {
    try {
      await logout();
    } catch {
      // Не блокируем выход из UI, даже если сеть/сервер подвели — cookies
      // всё равно будут перезаписаны при следующем логине.
    }
    await refresh();
    navigate("/docs");
  };

  return (
    <Header
      style={{
        color: "#fff",
        fontSize: 18,
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
      }}
    >
      <Link to="/docs" style={{ color: "#fff" }}>
        Genealogy MCP
      </Link>
      {!loading && (
        <Space>
          {session != null ? (
            <>
              <Typography.Text style={{ color: "#fff" }}>{session.login}</Typography.Text>
              <Link to="/settings" style={{ color: "#fff" }}>
                Настройки
              </Link>
              <Button type="link" style={{ color: "#fff" }} onClick={onLogout}>
                Выйти
              </Button>
            </>
          ) : (
            <Link to="/login" style={{ color: "#fff" }}>
              Войти
            </Link>
          )}
        </Space>
      )}
    </Header>
  );
}

// --- существующая часть, без изменений ---

function SettlementsTab() {
  const [items, setItems] = useState<AdminDivision[]>([]);
  const [parent, setParent] = useState<AdminDivision | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);

  const load = (q: AdminDivisionQuery) => {
    setLoading(true);
    setError(null);
    fetchAdminDivisions(q)
      .then(setItems)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load({ kind: "settlement", limit: MAX_PAGE_LIMIT });
  }, []);

  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q) {
      return;
    }
    setParent(null);
    setSearching(true);
    setError(null);
    searchAdminDivisions({ q, limit: MAX_PAGE_LIMIT })
      .then(setItems)
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const openChildren = (node: AdminDivision) => {
    setParent(node);
    load({ parent_id: node.id, limit: MAX_PAGE_LIMIT });
  };

  const toRoot = () => {
    setParent(null);
    load({ kind: "settlement", limit: MAX_PAGE_LIMIT });
  };

  return (
    <Card
      title={
        <>
          Населённые пункты
          {parent != null && (
            <Button type="link" onClick={toRoot} style={{ marginLeft: 12 }}>
              ← к корню
            </Button>
          )}
        </>
      }
    >
      <Input.Search
        placeholder="Поиск по названию…"
        allowClear
        enterButton
        loading={searching}
        onSearch={onSearch}
        style={{ marginBottom: 16 }}
      />
      {loading && <Spin />}
      {error != null && <Alert type="error" showIcon message={error} />}
      {items.length >= MAX_PAGE_LIMIT && (
        <Alert type="info" showIcon message={`Показаны первые ${MAX_PAGE_LIMIT}`} />
      )}
      {!loading && error == null && (
        <List
          dataSource={items}
          locale={{ emptyText: "Найдено пусто" }}
          renderItem={(s) => (
            <List.Item>
              <Typography.Text strong>{s.name}</Typography.Text>
              <Typography.Text type="secondary">{s.id}</Typography.Text>
              <Button type="link" onClick={() => openChildren(s)}>
                дети
              </Button>
            </List.Item>
          )}
        />
      )}
    </Card>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Routes>
          <Route path="/" element={<Navigate to="/docs" replace />} />
          <Route path="/docs" element={<AppContent />} />
          <Route path="/docs/:docPath*" element={<AppContent />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </SessionProvider>
    </BrowserRouter>
  );
}

function AppContent() {
  return (
    <Layout style={{ minHeight: "100vh" }}>
      <AppHeader />
      <Content style={{ padding: 24 }}>
        <Tabs
          defaultActiveKey="settlements"
          items={[
            {
              key: "settlements",
              label: (
                <>
                  <HomeOutlined /> Населённые пункты
                </>
              ),
              children: <SettlementsTab />,
            },
            {
              key: "docs",
              label: (
                <>
                  <BookOutlined /> Документация
                </>
              ),
              children: <DocsPanel />,
            },
          ]}
        />
      </Content>
    </Layout>
  );
}
