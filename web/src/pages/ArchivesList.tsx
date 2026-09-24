import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchArchives, searchArchives, MAX_PAGE_LIMIT, type Archive } from "../api";
import { useSession } from "../session";
import { CreateArchiveModal } from "./ArchiveForm";

// ArchivesList — «Архивы»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function ArchivesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Archive[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Archive[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Archive[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchArchives({ limit: MAX_PAGE_LIMIT, offset });
        all.push(...page);
        if (page.length < MAX_PAGE_LIMIT) {
          break;
        }
        offset += MAX_PAGE_LIMIT;
      }
      setItems(all);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить список");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q) {
      setSearchResults(null);
      return;
    }
    setSearching(true);
    setError(null);
    searchArchives({ q, limit: MAX_PAGE_LIMIT })
      .then(setSearchResults)
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const onSearchChange = (value: string) => {
    if (value.trim() === "") {
      setSearchResults(null);
    }
  };

  const shown = searchResults ?? items;

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Данные</Link> }, { title: "Архивы" }]}
      />
      <Card
        title="Архивы"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по названию…"
          allowClear
          enterButton
          loading={searching}
          onSearch={onSearch}
          onChange={(e) => onSearchChange(e.target.value)}
          style={{ marginBottom: 16 }}
        />
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {loading && searchResults == null ? (
          <Spin />
        ) : (
          <List
            dataSource={shown}
            locale={{ emptyText: "Список пуст" }}
            renderItem={(s) => (
              <List.Item>
                <Link to={`/archives/${s.id}`}>{s.name}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateArchiveModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/archives/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
