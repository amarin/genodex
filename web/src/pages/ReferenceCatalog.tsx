import { Breadcrumb, Card, List, Typography } from "antd";
import { Link } from "react-router-dom";

// REFERENCE_ENTRIES — справочники, встроенные в сервер (только чтение): по
// алфавиту названия, как CATALOG_ENTRIES раздела «Данные».
const REFERENCE_ENTRIES: { label: string; path: string }[] = [
  { label: "Типы административного деления", path: "/reference/division-types" },
];

const SORTED_ENTRIES = [...REFERENCE_ENTRIES].sort((a, b) => a.label.localeCompare(b.label, "ru"));

export default function ReferenceCatalog() {
  return (
    <>
      <Breadcrumb style={{ marginBottom: 16 }} items={[{ title: "Справочники" }]} />
      <Card title="Справочники">
        <List
          dataSource={SORTED_ENTRIES}
          renderItem={(entry) => (
            <List.Item>
              <Link to={entry.path}>
                <Typography.Text>{entry.label}</Typography.Text>
              </Link>
            </List.Item>
          )}
        />
      </Card>
    </>
  );
}
