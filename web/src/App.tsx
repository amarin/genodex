import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { useEffect, useState } from "react";
import { Alert, Card, Layout, List, Spin, Tabs, Typography } from "antd";
import { BookOutlined, HomeOutlined } from "@ant-design/icons";
import { fetchAdminDivisions, MAX_PAGE_LIMIT, type AdminDivision } from "./api";
import DocsPanel from "./docs-panel";

const { Header, Content } = Layout;

function SettlementsTab() {
  const [settlements, setSettlements] = useState<AdminDivision[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchAdminDivisions({ kind: "settlement", limit: MAX_PAGE_LIMIT })
      .then(setSettlements)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <Card title="Населённые пункты">
      {loading && <Spin />}
      {error != null && <Alert type="error" showIcon message={error} />}
      {settlements.length >= MAX_PAGE_LIMIT && (
        <Alert
          type="info"
          showIcon
          message={`Показаны первые ${MAX_PAGE_LIMIT} населённых пунктов`}
        />
      )}
      {!loading && error == null && (
        <List
          dataSource={settlements}
          locale={{ emptyText: "Населённых пунктов пока нет" }}
          renderItem={(s) => (
            <List.Item>
              <Typography.Text strong>{s.name}</Typography.Text>
              <Typography.Text type="secondary">{s.id}</Typography.Text>
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
      <Routes>
        <Route path="/" element={<Navigate to="/docs" replace />} />
        <Route path="/docs" element={<AppContent />} />
        <Route path="/docs/:docPath*" element={<AppContent />} />
      </Routes>
    </BrowserRouter>
  );
}

function AppContent() {
  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Header style={{ color: "#fff", fontSize: 18 }}>Genealogy MCP</Header>
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
