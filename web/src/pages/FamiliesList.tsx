import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchFamilies, searchFamilies, MAX_PAGE_LIMIT, type Family } from "../api";
import { useSession } from "../session";
import { CreateFamilyModal } from "./FamilyForm";

// FamiliesList — «Роды»: плоский список (сущность не иерархична, в отличие
// от AdminDivision — без Tree), пагинация по offset до короткой страницы
// (тот же приём, что DivisionsList.loadRoot — API уже отдаёт окнами,
// docs/data-model/entity-write.md §4). Поиск по названию временно подменяет
// список найденным. Клик по строке — переход на View.
export default function FamiliesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Family[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Family[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Family[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchFamilies({ limit: MAX_PAGE_LIMIT, offset });
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
    searchFamilies({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Данные</Link> }, { title: "Роды" }]}
      />
      <Card
        title="Роды"
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
            renderItem={(f) => (
              <List.Item>
                <Link to={`/families/${f.id}`}>{f.name}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateFamilyModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(f) => {
            setCreateOpen(false);
            navigate(`/families/${f.id}`);
          }}
        />
      </Card>
    </>
  );
}
