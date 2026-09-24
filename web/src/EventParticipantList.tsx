import { Button, Input, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { EventParticipant } from "./api";
import { PersonPicker } from "./PersonPicker";

// EventParticipantListEditor — редактор Event.Participants: список строк
// person_id (PersonPicker) + role (обязательно на бэкенде, requireText) +
// note (необязательно). Структурно повторяет SourceLinkListEditor (список
// строк + добавить/убрать по индексу), но проще PersonNameListEditor
// (подпроект 8): person_id — СТРОГАЯ ссылка на уже существующую персону
// (models.EventParticipant.PersonID, проверяется на сервере), а не мягкий
// TextRef с ref-preservation-если-текст-не-менялся — значит никакой
// ChurchView-style "снимок при появлении строки" логики не нужно
// (docs/data-model/entity-write.md §3.8: первая строгая ссылка внутри
// массива-объектов MCP/HTTP аргумента в программе).
export function EventParticipantListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: EventParticipant[];
  onChange: (next: EventParticipant[]) => void;
  addLabel: string;
}) {
  const setField = (i: number, patch: Partial<EventParticipant>) => {
    const next = value.slice();
    next[i] = { ...next[i], ...patch };
    onChange(next);
  };

  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));

  const add = () => onChange([...value, { person_id: "", role: "" }]);

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => (
        <Space key={i} wrap style={{ width: "100%" }}>
          <PersonPicker
            value={item.person_id}
            onChange={(v) => setField(i, { person_id: v })}
            placeholder="Персона"
          />
          <Input
            placeholder="Роль"
            value={item.role}
            onChange={(e) => setField(i, { role: e.target.value })}
            style={{ width: 160 }}
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
