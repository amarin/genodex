import { BrowserRouter, Routes, Route, Navigate, useParams } from "react-router-dom";
import { useEffect, useState } from "react";
import { Alert, Button, Card, Input, Layout, List, Spin, Tabs, Typography } from "antd";
import { BankOutlined, BookOutlined, HomeOutlined } from "@ant-design/icons";
import {
  fetchAdminDivisions,
  searchAdminDivisions,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type AdminDivisionQuery,
} from "./api";
import { SessionProvider } from "./session";
import { AppHeader } from "./AppHeader";
import DocsPanel from "./docs-panel";
import DivisionsList from "./pages/DivisionsList";
import DivisionView from "./pages/DivisionView";
import LoginPage from "./pages/Login";
import RegisterPage from "./pages/Register";
import SettingsPage from "./pages/Settings";

const { Content } = Layout;

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

// DivisionsTab — «Административное деление»: с id в URL показывает View
// конкретной единицы, без id — List (дерево от корня). Тот же приём, что
// DocsPanel использует для docPath — один компонент ветвится по параметру,
// а не отдельный <Route> на каждый режим внутри Tabs — сами роуты /divisions
// и /divisions/:id заведены в App() как два отдельных <Route>, ведущих на
// один и тот же <AppContent />.
function DivisionsTab() {
  const { id } = useParams<{ id?: string }>();
  return id != null ? <DivisionView /> : <DivisionsList />;
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Routes>
          <Route path="/" element={<Navigate to="/docs" replace />} />
          <Route path="/docs" element={<AppContent />} />
          <Route path="/docs/:docPath*" element={<AppContent />} />
          <Route path="/divisions" element={<AppContent />} />
          <Route path="/divisions/:id" element={<AppContent />} />
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
              key: "divisions",
              label: (
                <>
                  <BankOutlined /> Административное деление
                </>
              ),
              children: <DivisionsTab />,
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
