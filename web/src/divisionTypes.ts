import { useEffect, useState } from "react";
import {
  ADMIN_DIVISION_TYPE_LABELS,
  fetchAdminDivisionTypes,
  type AdminDivisionType,
  type AdminDivisionTypeInfo,
} from "./api";

// Правила вложенности типов живут на сервере (models.AdminDivisionType.CanContain)
// и приходят справочником GET /api/admin-division-types; здесь — только
// их применение в формах и проверка названия (она — подсказка UI, сервер
// названия не проверяет: «Погост» бывает и именем собственным села).

// useAdminDivisionTypes — справочник типов; null — ещё загружается или не
// загрузился (тогда формы работают без фильтра, сервер всё равно проверит).
export function useAdminDivisionTypes(): AdminDivisionTypeInfo[] | null {
  const [infos, setInfos] = useState<AdminDivisionTypeInfo[] | null>(null);
  useEffect(() => {
    let alive = true;
    fetchAdminDivisionTypes()
      .then((v) => alive && setInfos(v))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  return infos;
}

const ALL_TYPES = Object.keys(ADMIN_DIVISION_TYPE_LABELS) as AdminDivisionType[];

// allowedChildTypes — типы, допустимые внутри единицы типа parentType, по
// справочнику. Корень (null), незагруженный справочник и неизвестный тип —
// без ограничений.
export function allowedChildTypes(
  infos: AdminDivisionTypeInfo[] | null,
  parentType: string | null,
): AdminDivisionType[] {
  if (parentType == null || infos == null) {
    return ALL_TYPES;
  }
  return infos.find((i) => i.type === parentType)?.children ?? ALL_TYPES;
}

// Совпадает с models.AdminDivisionType.IsSettlement.
const SETTLEMENT_TYPES = new Set<AdminDivisionType>([
  "gorod",
  "poselok",
  "sloboda",
  "selo",
  "seltso",
  "derevnya",
  "hutor",
  "pogost",
  "stanitsa",
  "mestechko",
]);

export function isSettlementType(t: AdminDivisionType): boolean {
  return SETTLEMENT_TYPES.has(t);
}

export type ChildKinds = { divisions: boolean; settlements: boolean };

export function childKinds(types: AdminDivisionType[]): ChildKinds {
  return {
    divisions: types.some((t) => !isSettlementType(t)),
    settlements: types.some(isSettlementType),
  };
}

// childKindsPhrase — «деление или населённый пункт» / «деление» /
// «населённый пункт»; null — добавить нечего.
export function childKindsPhrase({ divisions, settlements }: ChildKinds): string | null {
  if (divisions && settlements) return "деление или населённый пункт";
  if (divisions) return "деление";
  if (settlements) return "населённый пункт";
  return null;
}

function normalizeWord(w: string): string {
  return w.toLowerCase().replace(/ё/g, "е");
}

// Слова-типы для проверки названия; «Иное» не считается — это не термин.
const TYPE_WORDS = new Map<string, string>(
  ALL_TYPES.filter((t) => t !== "other").map((t) => [
    normalizeWord(ADMIN_DIVISION_TYPE_LABELS[t]),
    ADMIN_DIVISION_TYPE_LABELS[t].toLowerCase(),
  ]),
);

// typeWordInName — первое слово названия, совпадающее с названием типа
// (целое слово, без учёта регистра и ё/е), в написании типа; null — нет.
// Название единицы — только имя собственное («Боровский», не «Боровский уезд»).
export function typeWordInName(name: string): string | null {
  for (const word of name.split(/[^\p{L}]+/u)) {
    if (word === "") continue;
    const found = TYPE_WORDS.get(normalizeWord(word));
    if (found != null) return found;
  }
  return null;
}

const typeWordRule = {
  validator: (_: unknown, value: string | undefined) => {
    const word = typeWordInName(value ?? "");
    return word == null
      ? Promise.resolve()
      : Promise.reject(
          new Error(`Название содержит тип «${word}» — укажите только имя, тип выбирается ниже`),
        );
  },
};

// DIVISION_NAME_RULES — правила antd Form.Item для поля «Название» единицы.
// Слово-тип в названии — только предупреждение: термин бывает и именем
// собственным (село «Погост»), поэтому запрещать его нельзя.
export const DIVISION_NAME_RULES = [
  { required: true, whitespace: true, message: "Введите название" },
  { ...typeWordRule, warningOnly: true },
];

export const DIVISION_NAME_PLACEHOLDER = "название, без типа";
