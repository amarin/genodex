import { useEffect, useState } from "react";
import { Alert, Card, Layout, List, Spin, Typography } from "antd";
import { fetchSettlements, type Settlement } from "./api";

const { Header, Content } = Layout;

export default function App() {
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
    <Layout style={{ minHeight: "100vh" }}>
      <Header style={{ color: "#fff", fontSize: 18 }}>Genealogy MCP</Header>
      <Content style={{ padding: 24 }}>
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
      </Content>
    </Layout>
  );
}