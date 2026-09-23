import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchArchiveDocuments, searchArchiveDocuments, MAX_PAGE_LIMIT, type ArchiveDocument } from "../api";
import { useSession } from "../session";
import { CreateArchiveDocumentModal } from "./ArchiveDocumentForm";

function documentLabel(d: ArchiveDocument): string {
  return d.title || d.id;
}

// ArchiveDocumentsList — «Архивные документы»: плоский список (в отличие от
// ArchiveNode — ArchiveDocument не иерархична), тот же
// пагинация-до-короткой-страницы + поиск-подменяет-список, что NotesList.tsx.
export default function ArchiveDocumentsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<ArchiveDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<ArchiveDocument[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: ArchiveDocument[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchArchiveDocuments({ limit: MAX_PAGE_LIMIT, offset });
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
    searchArchiveDocuments({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Архивные документы" }]}
      />
      <Card
        title="Архивные документы"
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
            renderItem={(d) => (
              <List.Item>
                <Link to={`/archive-documents/${d.id}`}>{documentLabel(d)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateArchiveDocumentModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(d) => {
            setCreateOpen(false);
            navigate(`/archive-documents/${d.id}`);
          }}
        />
      </Card>
    </>
  );
}
