import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchPeople, searchPeople, MAX_PAGE_LIMIT, type Person } from "../api";
import { useSession } from "../session";
import { pickDisplayName, formatPersonName } from "../PersonNameList";
import { CreatePersonModal } from "./PersonForm";

// personLabel — строка списка: "main" имя, иначе первое из Names,
// отформатированное как "Фамилия Имя Отчество" (PersonNameList.formatPersonName);
// если Names пуст или все части пустые — id (docs/data-model/entity-write.md §3.7).
function personLabel(p: Person): string {
  const name = pickDisplayName(p.names);
  const formatted = name != null ? formatPersonName(name) : "";
  return formatted !== "" ? formatted : p.id;
}

// PeopleList — «Персоны»: плоский список (как FamiliesList — не иерархична),
// пагинация по offset до короткой страницы, поиск временно подменяет список
// найденным. Клик по строке — переход на View.
export default function PeopleList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Person[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Person[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Person[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchPeople({ limit: MAX_PAGE_LIMIT, offset });
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
    searchPeople({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Персоны" }]}
      />
      <Card
        title="Персоны"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по фамилии/имени/отчеству…"
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
            renderItem={(p) => (
              <List.Item>
                <Link to={`/people/${p.id}`}>{personLabel(p)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreatePersonModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(p) => {
            setCreateOpen(false);
            navigate(`/people/${p.id}`);
          }}
        />
      </Card>
    </>
  );
}
