import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, List, Spin } from "antd";
import { fetchRelations, MAX_PAGE_LIMIT, relationKindLabel, type Relation } from "../api";
import { useSession } from "../session";
import { usePersonOptions } from "../PersonPicker";
import { personDisplayName } from "../PersonNameList";
import { CreateRelationModal } from "./RelationForm";

// RelationsList — «Связи»: плоский список БЕЗ поиска (Relation не имеет
// /search — его search-индекс всегда пуст по конструкции,
// docs/data-model/entity-write.md §3.8) — пагинация по offset до короткой
// страницы, как у прочих плоских списков, просто без Input.Search и без
// онсёрч-подмены списка.
export default function RelationsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Relation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const people = usePersonOptions();

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Relation[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchRelations({ limit: MAX_PAGE_LIMIT, offset });
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

  const personLabel = (id: string) => {
    const p = people.find((person) => person.id === id);
    return p != null ? personDisplayName(p) : id;
  };

  const relationLabel = (r: Relation) => {
    const kind = relationKindLabel(r.kind) + (r.kind === "associate" && r.rel_type ? ` (${r.rel_type})` : "");
    return `${kind}: ${personLabel(r.person_a)} — ${personLabel(r.person_b)}`;
  };

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Связи" }]}
      />
      <Card
        title="Связи"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {loading ? (
          <Spin />
        ) : (
          <List
            dataSource={items}
            locale={{ emptyText: "Список пуст" }}
            renderItem={(r) => (
              <List.Item>
                <Link to={`/relations/${r.id}`}>{relationLabel(r)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateRelationModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(r) => {
            setCreateOpen(false);
            navigate(`/relations/${r.id}`);
          }}
        />
      </Card>
    </>
  );
}
