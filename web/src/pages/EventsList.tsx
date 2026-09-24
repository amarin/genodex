import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchEvents, searchEvents, MAX_PAGE_LIMIT, type Event } from "../api";
import { useSession } from "../session";
import { formatFactDate } from "../FactDateEditor";
import { CreateEventModal } from "./EventForm";

// eventLabel — вид события + место (если есть) + дата (если есть), по
// образцу personNameLine/PersonView.tsx — короткая строка для списка и
// для заголовка EventView.
export function eventLabel(e: Event): string {
  const parts = [e.type];
  if (e.place?.text) {
    parts.push(e.place.text);
  }
  const date = formatFactDate(e.date);
  if (date !== "—") {
    parts.push(date);
  }
  return parts.join(" — ");
}

// EventsList — «События»: единственная сущность подпроекта 9 с /search
// (место-текст-префиксный поиск, docs/data-model/entity-write.md §3.8) —
// структурно идентична FamiliesList/PeopleList (поиск временно подменяет
// список найденным).
export default function EventsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Event[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Event[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchEvents({ limit: MAX_PAGE_LIMIT, offset });
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
    searchEvents({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "События" }]}
      />
      <Card
        title="События"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по началу текста места…"
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
            renderItem={(e) => (
              <List.Item>
                <Link to={`/events/${e.id}`}>{eventLabel(e)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateEventModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(e) => {
            setCreateOpen(false);
            navigate(`/events/${e.id}`);
          }}
        />
      </Card>
    </>
  );
}
