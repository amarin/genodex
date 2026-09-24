import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, List, Spin } from "antd";
import {
  adminDivisionTypeLabel,
  fetchAdminDivisions,
  fetchResidences,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type Residence,
} from "../api";
import { useSession } from "../session";
import { usePersonOptions } from "../PersonPicker";
import { personDisplayName } from "../PersonNameList";
import { CreateResidenceModal } from "./ResidenceForm";

// ResidencesList — «Проживания»: плоский список БЕЗ поиска (Residence, как
// Relation, не имеет /search — пустой search-индекс по конструкции,
// docs/data-model/entity-write.md §3.8) — пагинация по offset до короткой
// страницы.
export default function ResidencesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Residence[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [divisions, setDivisions] = useState<AdminDivision[]>([]);
  const people = usePersonOptions();

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Residence[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchResidences({ limit: MAX_PAGE_LIMIT, offset });
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

  useEffect(() => {
    fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
      .then(setDivisions)
      .catch(() => setDivisions([]));
  }, []);

  const personLabel = (id: string) => {
    const p = people.find((person) => person.id === id);
    return p != null ? personDisplayName(p) : id;
  };

  const placeLabel = (id: string) => {
    const d = divisions.find((division) => division.id === id);
    return d != null ? `${d.name} (${adminDivisionTypeLabel(d.type)})` : id;
  };

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Данные</Link> }, { title: "Проживания" }]}
      />
      <Card
        title="Проживания"
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
                <Link to={`/residences/${r.id}`}>
                  {personLabel(r.person_id)} — {placeLabel(r.place_id)}
                </Link>
              </List.Item>
            )}
          />
        )}
        <CreateResidenceModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(r) => {
            setCreateOpen(false);
            navigate(`/residences/${r.id}`);
          }}
        />
      </Card>
    </>
  );
}
