import { useEffect, useState } from "react";
import { Alert, Card, Layout, List, Spin, Tabs, Typography } from "antd";
import { BookOutlined, HomeOutlined } from "@ant-design/icons";
import { fetchSettlements, type Settlement } from "./api";
import DocsPanel from "./docs-panel";

const { Header, Content } = Layout;

function SettlementsTab() {
  const [settlements, setSettlements] = useState<Settlement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchSettlements()
      .then(setSettlements)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <Card title="Населённые пункты">
      {loading && <Spin />}
      {error != null && <Alert type="error" showIcon message={error} />}
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