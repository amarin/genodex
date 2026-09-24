import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchNotes, searchNotes, MAX_PAGE_LIMIT, type Note } from "../api";
import { useSession } from "../session";
import { CreateNoteModal } from "./NoteForm";

function noteLabel(n: Note): string {
  return n.title || n.text?.slice(0, 80) || n.id;
}

// NotesList — «Заметки»: плоский список (иерархия parent_id не показывается
// деревом в v1, обсуждение подпроекта 4) — та же пагинация-до-короткой-
// страницы и поиск-подменяет-список, что у ArchivesList.
export default function NotesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Note[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Note[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Note[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchNotes({ limit: MAX_PAGE_LIMIT, offset });
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
    searchNotes({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Данные</Link> }, { title: "Заметки" }]}
      />
      <Card
        title="Заметки"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по заголовку…"
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
            renderItem={(n) => (
              <List.Item>
                <Link to={`/notes/${n.id}`}>{noteLabel(n)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateNoteModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(n) => {
            setCreateOpen(false);
            navigate(`/notes/${n.id}`);
          }}
        />
      </Card>
    </>
  );
}
