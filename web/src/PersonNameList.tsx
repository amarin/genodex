import { useRef, useState } from "react";
import { Button, Divider, Input, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { PersonName, PersonNameType, TextRef } from "./api";
import { FactDateEditor } from "./FactDateEditor";

// PERSON_NAME_TYPE_OPTIONS — models.PersonNameType (internal/models/person_name_type.go).
export const PERSON_NAME_TYPE_OPTIONS: { value: PersonNameType; label: string }[] = [
  { value: "", label: "Не указано" },
  { value: "main", label: "основное" },
  { value: "birth", label: "при рождении" },
  { value: "married", label: "по браку" },
  { value: "changed", label: "изменённое" },
  { value: "pseudonym", label: "псевдоним" },
];

export function personNameTypeLabel(t: string | undefined): string {
  return PERSON_NAME_TYPE_OPTIONS.find((o) => o.value === (t ?? ""))?.label ?? (t || "");
}

// formatPersonName — "Фамилия Имя Отчество" (пропускает пустые части),
// общая для PeopleList (строка списка) и PersonView (просмотр имени).
export function formatPersonName(n: PersonName): string {
  return [n.surname.text, n.given.text, n.patronymic.text]
    .map((s) => s.trim())
    .filter((s) => s !== "")
    .join(" ");
}

// pickDisplayName — имя типа "main", иначе первое из списка; null, если
// список пуст.
export function pickDisplayName(names: PersonName[]): PersonName | null {
  if (names.length === 0) {
    return null;
  }
  return names.find((n) => n.type === "main") ?? names[0];
}

const EMPTY_TEXT_REF: TextRef = { text: "" };

function emptyPersonName(): PersonName {
  return {
    type: "",
    surname: { ...EMPTY_TEXT_REF },
    given: { ...EMPTY_TEXT_REF },
    patronymic: { ...EMPTY_TEXT_REF },
    prefix: "",
    suffix: "",
    since: null,
    until: null,
  };
}

interface RowMeta {
  id: number;
  // base — исходный (загруженный) PersonName этой строки на момент её
  // появления в редакторе: точка отсчёта для решения "сохранить ref или
  // сбросить" (ChurchView-style, docs/data-model/entity-write.md §4) —
  // берётся один раз при монтировании/добавлении строки, а не при каждом
  // изменении value, иначе ref терялся бы сразу после первого чтения.
  base: PersonName;
}

// PersonNameListEditor — редактор Person.names: первая в проекте вложенная
// подформа «массив объектов» (не массив строк/TextRef, как
// TextRefListEditor, и не массив с одной строгой ссылкой, как
// SourceLinkListEditor) — структурно та же форма "список строк, добавить/
// убрать", но каждая строка богаче: вид имени (Select) + три TextRef-поля
// (Фамилия/Имя/Отчество — простые Input, БЕЗ picker'а, тот же принцип, что
// Church.Parish: строгие FK получают Select, мягкий TextRef — нет) + два
// служебных текстовых поля (Приставка/Суффикс) + период действия
// (FactDateEditor since/until, тот же компонент, что у Parish).
//
// Сохранение ref при неизменном тексте — независимо для surname/given/
// patronymic внутри каждой строки, по образцу ChurchView.onSave
// (parishText/parishHasRef): если текст поля не отличается от того, что
// было при появлении строки в редакторе, и то исходное значение несло ref —
// наружу уходит исходный объект {text, ref, type} без изменений; иначе —
// свежий {text}, без ref/type (пользователь напечатал что-то другое —
// прежняя ссылка на конкретную словарную запись больше не действует).
export function PersonNameListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: PersonName[];
  onChange: (next: PersonName[]) => void;
  addLabel: string;
}) {
  // Снимок «во что редактор был инициализирован» — один раз при монтировании
  // (компонент пересоздаётся при каждом входе в режим редактирования/
  // открытии модалки создания, см. PersonForm.tsx/PersonView.tsx, так что
  // это соответствует «на момент появления строки»).
  const [rowMeta, setRowMeta] = useState<RowMeta[]>(() => value.map((n, i) => ({ id: i, base: n })));
  const nextId = useRef(value.length);

  const setRow = (i: number, patch: Partial<PersonName>) => {
    const next = value.slice();
    next[i] = { ...next[i], ...patch };
    onChange(next);
  };

  const setTextRefField = (i: number, field: "surname" | "given" | "patronymic", text: string) => {
    const base = rowMeta[i]?.base;
    const baseField = base ? base[field] : undefined;
    const hasRef = baseField != null && baseField.ref != null && baseField.ref !== "";
    const nextField: TextRef = hasRef && text === baseField.text ? baseField : { text };
    setRow(i, { [field]: nextField } as Partial<PersonName>);
  };

  const add = () => {
    const fresh = emptyPersonName();
    onChange([...value, fresh]);
    setRowMeta([...rowMeta, { id: nextId.current++, base: fresh }]);
  };

  const remove = (i: number) => {
    onChange(value.filter((_, idx) => idx !== i));
    setRowMeta(rowMeta.filter((_, idx) => idx !== i));
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => (
        <div key={rowMeta[i]?.id ?? i}>
          <Space direction="vertical" style={{ width: "100%" }}>
            <Space wrap style={{ width: "100%" }}>
              <Select
                style={{ width: 160 }}
                placeholder="Вид имени"
                options={PERSON_NAME_TYPE_OPTIONS}
                value={item.type ?? ""}
                onChange={(v) => setRow(i, { type: v })}
              />
              <Input
                placeholder="Фамилия"
                value={item.surname.text}
                onChange={(e) => setTextRefField(i, "surname", e.target.value)}
                style={{ width: 160 }}
              />
              <Input
                placeholder="Имя"
                value={item.given.text}
                onChange={(e) => setTextRefField(i, "given", e.target.value)}
                style={{ width: 160 }}
              />
              <Input
                placeholder="Отчество"
                value={item.patronymic.text}
                onChange={(e) => setTextRefField(i, "patronymic", e.target.value)}
                style={{ width: 160 }}
              />
              <Input
                placeholder="Приставка"
                value={item.prefix}
                onChange={(e) => setRow(i, { prefix: e.target.value })}
                style={{ width: 120 }}
              />
              <Input
                placeholder="Суффикс"
                value={item.suffix}
                onChange={(e) => setRow(i, { suffix: e.target.value })}
                style={{ width: 120 }}
              />
              <MinusCircleOutlined onClick={() => remove(i)} />
            </Space>
            <Space wrap style={{ width: "100%" }}>
              <FactDateEditor
                value={item.since ?? null}
                onChange={(since) => setRow(i, { since })}
                addLabel="+ действует с"
              />
              <FactDateEditor
                value={item.until ?? null}
                onChange={(until) => setRow(i, { until })}
                addLabel="+ действует по"
              />
            </Space>
          </Space>
          <Divider style={{ margin: "12px 0" }} />
        </div>
      ))}
      <Button type="dashed" onClick={add} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    </Space>
  );
}
