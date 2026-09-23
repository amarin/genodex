import { Button, InputNumber, Select, Space, Typography } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { FactCalendar, FactDate, FactModifier, FactPrecision } from "./api";

const PRECISION_OPTIONS: { value: FactPrecision; label: string }[] = [
  { value: "unknown", label: "Неизвестна" },
  { value: "year", label: "Год" },
  { value: "month", label: "Месяц" },
  { value: "day", label: "День" },
];

const MODIFIER_OPTIONS: { value: FactModifier; label: string }[] = [
  { value: "exact", label: "Точно" },
  { value: "approx", label: "Около" },
  { value: "before", label: "До" },
  { value: "after", label: "После" },
  { value: "between", label: "Между" },
];

const CALENDAR_OPTIONS: { value: FactCalendar; label: string }[] = [
  { value: "", label: "Не указан" },
  { value: "gregorian", label: "Григорианский" },
  { value: "julian", label: "Юлианский" },
  { value: "unknown", label: "Неизвестен" },
];

const EMPTY_DATE: FactDate = { year: 0, precision: "year", modifier: "exact" };

const CALENDAR_SUFFIX: Record<string, string> = { julian: " ст. ст.", gregorian: " н. ст." };

// formatFactDate — читаемое текстовое представление даты для режима
// просмотра (аналог models.FactDate.String() на бэкенде, упрощённый —
// только для отображения, не для парсинга обратно).
export function formatFactDate(d: FactDate | null | undefined): string {
  if (d == null || d.precision === "unknown") {
    return "—";
  }

  const pad = (n: number) => String(n).padStart(2, "0");
  let base = String(d.year);
  if (d.precision === "month" || d.precision === "day") {
    base += `-${pad(d.month ?? 0)}`;
  }
  if (d.precision === "day") {
    base += `-${pad(d.day ?? 0)}`;
  }

  let text: string;
  switch (d.modifier) {
    case "approx":
      text = `около ${base}`;
      break;
    case "before":
      text = `до ${base}`;
      break;
    case "after":
      text = `после ${base}`;
      break;
    case "between": {
      let hi = String(d.year_to ?? 0);
      if (d.month_to) {
        hi += `-${pad(d.month_to)}`;
      }
      if (d.day_to) {
        hi += `-${pad(d.day_to)}`;
      }
      text = `между ${base} и ${hi}`;
      break;
    }
    default:
      text = base;
  }

  return text + (CALENDAR_SUFFIX[d.calendar ?? ""] ?? "");
}

// FactDateEditor — редактор структурированной даты (models.FactDate): год/
// месяц/день по точности, формулировка (точно/около/до/после/между),
// календарь, верхняя граница периода для modifier=between. Первое появление
// в проекте (Parish.Since/Until) — переиспользуется в Family/Person/Event
// (docs/data-model/entity-write.md §2, подпроекты 7-9). Верхняя граница
// («До:») использует ту же точность, что и нижняя — упрощение v1: модель
// допускает разную точность, но для одной формы это не нужно.
export function FactDateEditor({
  value,
  onChange,
  addLabel,
}: {
  value: FactDate | null;
  onChange: (next: FactDate | null) => void;
  addLabel: string;
}) {
  if (value == null) {
    return (
      <Button type="dashed" onClick={() => onChange(EMPTY_DATE)} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    );
  }

  const set = (patch: Partial<FactDate>) => onChange({ ...value, ...patch });

  const setPrecision = (precision: FactPrecision) => {
    const patch: Partial<FactDate> = { precision };
    if (precision === "unknown") {
      patch.year = 0;
      patch.month = 0;
      patch.day = 0;
      patch.modifier = "exact";
      patch.year_to = 0;
      patch.month_to = 0;
      patch.day_to = 0;
    } else if (precision === "year") {
      patch.month = 0;
      patch.day = 0;
      patch.month_to = 0;
      patch.day_to = 0;
    } else if (precision === "month") {
      patch.day = 0;
      patch.day_to = 0;
    }
    set(patch);
  };

  const setModifier = (modifier: FactModifier) => {
    if (modifier === "between") {
      set({ modifier });
    } else {
      set({ modifier, year_to: 0, month_to: 0, day_to: 0 });
    }
  };

  const needMonth = value.precision === "month" || value.precision === "day";
  const needDay = value.precision === "day";
  const isBetween = value.modifier === "between";

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space wrap>
        <Select
          style={{ width: 140 }}
          value={value.precision}
          options={PRECISION_OPTIONS}
          onChange={setPrecision}
        />
        {value.precision !== "unknown" && (
          <>
            <InputNumber
              placeholder="Год"
              min={1}
              max={9999}
              value={value.year || undefined}
              onChange={(v) => set({ year: v ?? 0 })}
              style={{ width: 90 }}
            />
            {needMonth && (
              <InputNumber
                placeholder="Месяц"
                min={1}
                max={12}
                value={value.month || undefined}
                onChange={(v) => set({ month: v ?? 0 })}
                style={{ width: 80 }}
              />
            )}
            {needDay && (
              <InputNumber
                placeholder="День"
                min={1}
                max={31}
                value={value.day || undefined}
                onChange={(v) => set({ day: v ?? 0 })}
                style={{ width: 80 }}
              />
            )}
          </>
        )}
        <Select
          style={{ width: 120 }}
          value={value.modifier}
          options={MODIFIER_OPTIONS}
          disabled={value.precision === "unknown"}
          onChange={setModifier}
        />
        <Select
          style={{ width: 150 }}
          value={value.calendar ?? ""}
          options={CALENDAR_OPTIONS}
          onChange={(calendar: FactCalendar) => set({ calendar })}
        />
        <MinusCircleOutlined onClick={() => onChange(null)} />
      </Space>
      {isBetween && (
        <Space wrap>
          <Typography.Text type="secondary">До:</Typography.Text>
          <InputNumber
            placeholder="Год"
            min={1}
            max={9999}
            value={value.year_to || undefined}
            onChange={(v) => set({ year_to: v ?? 0 })}
            style={{ width: 90 }}
          />
          {needMonth && (
            <InputNumber
              placeholder="Месяц"
              min={1}
              max={12}
              value={value.month_to || undefined}
              onChange={(v) => set({ month_to: v ?? 0 })}
              style={{ width: 80 }}
            />
          )}
          {needDay && (
            <InputNumber
              placeholder="День"
              min={1}
              max={31}
              value={value.day_to || undefined}
              onChange={(v) => set({ day_to: v ?? 0 })}
              style={{ width: 80 }}
            />
          )}
        </Space>
      )}
    </Space>
  );
}
