import { useEffect, useState } from "react";
import { Button, Input, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import { fetchCitations, type Citation, type Reliability, type SourceLink } from "./api";

const RELIABILITY_OPTIONS: { value: Reliability; label: string }[] = [
  { value: "primary", label: "Первичный" },
  { value: "contemporary", label: "Современник" },
  { value: "memory", label: "Со слов/по памяти" },
  { value: "indirect", label: "Косвенный" },
  { value: "unknown", label: "Неизвестна" },
];

function citationLabel(c: Citation): string {
  return c.text || c.id;
}

// useCitationOptions — заполняет Select цитат для SourceLink.citation_id, по
// образцу useRepositoryOptions/ArchiveForm.tsx.
function useCitationOptions() {
  const [citations, setCitations] = useState<Citation[]>([]);

  useEffect(() => {
    fetchCitations({ limit: 500 })
      .then(setCitations)
      .catch(() => setCitations([]));
  }, []);

  return citations.map((c) => ({ value: c.id, label: citationLabel(c) }));
}

// SourceLinkListEditor — общий редактор списка доказательств (Sources —
// подпроект 5: было read-only у Repository/Church/Parish/Archive/Note/
// AdministrativeDivision, теперь редактируется везде, где есть). Каждая
// строка — ссылка на цитату (Select с поиском) + достоверность именно этого
// утверждения по этой цитате + роль + заметка. target_type/target_id не
// редактируются и не отправляются — владелец подставляется сервером из
// контекста (см. api.ts's SourceLink и бэкенд-комментарий в
// transport.SourceLink.Model()).
export function SourceLinkListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: SourceLink[];
  onChange: (next: SourceLink[]) => void;
  addLabel: string;
}) {
  const citationOptions = useCitationOptions();

  const setField = (i: number, patch: Partial<SourceLink>) => {
    const next = value.slice();
    next[i] = { ...next[i], ...patch };
    onChange(next);
  };

  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));

  const add = () => onChange([...value, { citation_id: "" }]);

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => (
        <Space key={i} wrap style={{ width: "100%" }}>
          <Select
            style={{ width: 260 }}
            placeholder="Цитата"
            options={citationOptions}
            value={item.citation_id || undefined}
            onChange={(v) => setField(i, { citation_id: v })}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
          <Select
            style={{ width: 180 }}
            placeholder="Достоверность"
            allowClear
            options={RELIABILITY_OPTIONS}
            value={(item.reliability as Reliability) || undefined}
            onChange={(v) => setField(i, { reliability: v })}
          />
          <Input
            placeholder="Роль"
            value={item.role}
            onChange={(e) => setField(i, { role: e.target.value })}
            style={{ width: 140 }}
          />
          <Input
            placeholder="Заметка"
            value={item.note}
            onChange={(e) => setField(i, { note: e.target.value })}
            style={{ width: 200 }}
          />
          <MinusCircleOutlined onClick={() => remove(i)} />
        </Space>
      ))}
      <Button type="dashed" onClick={add} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    </Space>
  );
}
