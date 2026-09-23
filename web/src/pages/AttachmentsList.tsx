import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchAttachments, searchAttachments, MAX_PAGE_LIMIT, type Attachment } from "../api";
import { useSession } from "../session";
import { CreateAttachmentModal } from "./AttachmentForm";

function attachmentLabel(a: Attachment): string {
  return a.filename || a.uri || a.id;
}

// AttachmentsList — «Вложения»: плоский список, та же пагинация-до-короткой-
// страницы и поиск-подменяет-список, что у ArchivesList/NotesList.
export default function AttachmentsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Attachment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Attachment[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Attachment[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchAttachments({ limit: MAX_PAGE_LIMIT, offset });
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
    searchAttachments({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Вложения" }]}
      />
      <Card
        title="Вложения"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по имени файла или заметке…"
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
            renderItem={(a) => (
              <List.Item>
                <Link to={`/attachments/${a.id}`}>{attachmentLabel(a)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateAttachmentModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(a) => {
            setCreateOpen(false);
            navigate(`/attachments/${a.id}`);
          }}
        />
      </Card>
    </>
  );
}
