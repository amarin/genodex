# Веб-CRUD для административного деления: план

Первая веб-запись поверх готового API этапа C
(`internal/httpapi/division_write.go` — `POST`/`PUT`/`DELETE
/api/admin-divisions...`, уже требует `requireFull`). Сейчас на фронте
только чтение: вкладка «Населённые пункты» — плоский список `kind=settlement`
без записи. Формат плана — как у прочих проходов (`2026-09-22-auth-*.md`).
Исполняется в `main` подрядными коммитами.

## Goal

Дать владельцу управлять всей иерархией административного деления
(не только населёнными пунктами) прямо в браузере: список/дерево, просмотр
с переходом по связям, создание, редактирование, удаление — без выхода в
`curl`/MCP.

## Предпосылка: дизайн, обсуждённый и согласованный с пользователем

Обсуждение (brainstorming, architectural path) зафиксировало:

1. **Объём** — управляется вся иерархия `AdminDivisionType`
   (governorate/district/volost/other/gorod/selo/derevnya/hutor/pogost/
   stanitsa/mestechko), не только населённые пункты. Существующая вкладка
   «Населённые пункты» (`kind=settlement`, плоский список) не трогается —
   остаётся отдельной вкладкой.
2. **Страницы** — паттерн List/View на будущее (другим типам сущностей
   пока нечего показывать записью — API есть только у AdminDivision,
   строить общий фреймворк сейчас — YAGNI), но сама раскладка файлов
   рассчитана на копирование: `/divisions` (List, дерево от корня) и
   `/divisions/:id` (View — поля + дочерние единицы + переключатель
   view/edit). Один компонент (`DivisionsTab`) ветвится по параметру `id`
   из `useParams`, как уже делает `DocsPanel` для `docPath` — не отдельный
   `<Route>` на каждый режим.
3. **Save UX** — явная кнопка «Сохранить» в edit-режиме (не автосейв по
   полям): `PUT /api/admin-divisions/{id}` — это **полная замена**
   name/type/parent_id разом, с проверкой цикла по всей тройке на сервере
   (`internal/usecases/update_division/scenario.go`) — раздельного PATCH
   нет, автосейв по одному полю означал бы либо новый API, либо отправку
   всех трёх полей на каждое изменение одного.
4. **List** — дерево от корня. **Уточнение роадмапа** (обнаружено живой
   проверкой при выполнении плана, см. раздел «Предпосылка» ниже, пункт про
   исправление): комментарий `models.DivisionQuery.ParentID` («nil —
   корень», `internal/models/query.go:67`) вводит в заблуждение —
   фактически `ListDivisions` при `ParentID == nil` (`internal/usecases/
   list_divisions/scenario.go:31-33`) отдаёт ВЕСЬ список единиц без
   фильтра по родителю (`Matches()` вообще не проверяет `ParentID`), а не
   только корневые. Настоящей корневой фильтрации на бэке нет. Решение —
   фильтровать на клиенте: `fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })`
   отдаёт полный список (в пределах окна), из него берём только записи с
   `parent_id === null`. Дети подгружаются по клику на раскрывашку (`antd
   Tree` + `loadData`, там `parent_id` передаётся явно и бэк действительно
   фильтрует — `listChildren`, та же `scenario.go`). Клик по названию узла —
   переход на View. Поиск по названию временно подменяет дерево плоским
   списком найденного (как у «Населённые пункты»).
5. **Вкладка** называется «Административное деление» (не «Деления» —
   отклонено пользователем как неудачное).
6. **Конфликт удаления** (409, `*models.InUseError`) — модалка со списком
   `Referrers` (тип + id, без локализации типа сущности — 21 тип, полный
   словарь на фронте сейчас не оправдан, тот же формат, что и в
   `InUseError.Error()` на бэке).

**Отклонённая альтернатива** (обсуждалась и отклонена в диалоге, не
переносится в план): построить сразу общий framework на List/View/Edit для
всех типов сущностей — отклонено явно пользователем как преждевременное
(«Только AdminDivision, но с заделом на паттерн»), поскольку записи для
остальных 20 типов ещё не существует даже на бэке.

**Известное ограничение, не в объёме этого плана**: `Tabs` в `AppContent`
использует `defaultActiveKey="settlements"` (неконтролируемое состояние) —
прямой заход по ссылке на `/divisions` или `/divisions/:id` (свежая загрузка
страницы, не переход изнутри приложения) показывает активной вкладку
«Населённые пункты», пока пользователь не кликнет по «Административное
деление» — тогда `DivisionsTab` уже корректно покажет нужный режим по `id`
из URL. Это тот же существующий эффект, что и у `/docs/:docPath*` сегодня
(тоже не подсвечивает вкладку «Документация» при прямом заходе) — не
регрессия этого плана, чинить эту синхронизацию Tabs↔URL вне объёма.

**Живая проверка перед этим планом** (весь код ниже уже применён к рабочему
дереву и проверен): `npm run typecheck`/`npm run build` чисты; в браузере
через реальный `genodex serve -web dev` с чистой БД — создание корневой
единицы (название + выбор типа через `Select`) прошло полностью, включая
персист и корректное отображение; дерево верно подгружает и помечает узел
листом при пустом списке детей; клик по названию узла переводит на View;
edit-режим верно предзаполняется и `PUT` корректно применяет изменение
названия; удаление корневой единицы делает `DELETE` и редиректит на
`/divisions`. Повторные попытки автоматически кликнуть опцию в открытом
`Select` были нестабильны (инструмент браузера иногда не долетал кликом до
виртуализированного пункта списка) — сам механизм (`Form` + `Select
options={TYPE_OPTIONS}`) при этом уже доказанно работает (первое создание
прошло полностью), так что этот код ниже не переоткрывался.

**Находка при выполнении плана (Задача 3, живая проверка контроллером)**:
первый прогон выше создавал только ОДНУ корневую единицу без детей — с
одним таким узлом поведение «отдать всё» и «отдать только корень»
неотличимо на экране. Как только появился второй уровень иерархии
(родитель + ребёнок), список на `/divisions` показал ребёнка тоже в
корне — вскрылось, что `fetchAdminDivisions({})` без `parent_id` отдаёт
весь список, а не только корневые записи (см. «Уточнение роадмапа» в
пункте 4 выше). Заведён отдельный фикс-раунд на Задачу 2 (`DivisionsList.
loadRoot` получил клиентский фильтр `parent_id === null`) — код в Шаге 2.2
ниже уже отражает исправленную версию.

## Задача 1. Веб-клиент: `authFetch` наружу + запись делений

**Файлы:**
- Изменить: `web/src/auth.ts`, `web/src/api.ts`

- [ ] Шаг 1.1. `web/src/auth.ts` — замени блок `ApiError` (после `Invite`
  interface, перед `NO_REFRESH_RETRY_PATHS`):
  ```typescript
  // ApiErrorReferrer — элемент Referrers из transport.InUseErrorBody (409 на
  // удалении занятой единицы).
  export interface ApiErrorReferrer {
    type: string;
    id: string;
  }

  // ApiError — тело {error, field?, referrers?} из writeAuthError/writeJSON
  // (Go). field заполнен только для *auth.ValidationError (422); referrers —
  // только для *models.InUseError (409, internal/transport/errors.go).
  export class ApiError extends Error {
    status: number;
    field?: string;
    referrers?: ApiErrorReferrer[];

    constructor(status: number, message: string, field?: string, referrers?: ApiErrorReferrer[]) {
      super(message);
      this.status = status;
      this.field = field;
      this.referrers = referrers;
    }
  }
  ```
  (заменяет прежний блок `ApiError` без `referrers`).

- [ ] Шаг 1.2. `web/src/auth.ts` — сделай `authFetch` экспортируемой (нужна
  в `api.ts` для чтения одной единицы и записи): замени
  ```typescript
  async function authFetch<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
  ```
  на
  ```typescript
  export async function authFetch<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
  ```
  Тело функции не меняется, кроме блока разбора ошибки — замени
  ```typescript
  if (!resp.ok) {
    let body: { error?: string; field?: string } = {};
    try {
      body = await resp.json();
    } catch {
      // тело не JSON (не должно происходить у /api/auth/*, но не валим клиент)
    }
    throw new ApiError(resp.status, body.error ?? `Ошибка ${resp.status}`, body.field);
  }
  ```
  на
  ```typescript
  if (!resp.ok) {
    let body: { error?: string; field?: string; referrers?: ApiErrorReferrer[] } = {};
    try {
      body = await resp.json();
    } catch {
      // тело не JSON (не должно происходить у /api/auth/*, но не валим клиент)
    }
    throw new ApiError(resp.status, body.error ?? `Ошибка ${resp.status}`, body.field, body.referrers);
  }
  ```

- [ ] Шаг 1.3. `web/src/api.ts` — добавь импорт в самое начало файла (перед
  `export interface AdminDivision`):
  ```typescript
  import { authFetch } from "./auth";

  ```

- [ ] Шаг 1.4. `web/src/api.ts` — сразу после блока `export interface
  AdminDivision { ... }` добавь типы и словарь меток:
  ```typescript
  // AdminDivisionType — models.AdminDivisionType (internal/models/administrative_division_type.go).
  export type AdminDivisionType =
    | "governorate"
    | "district"
    | "volost"
    | "other"
    | "gorod"
    | "selo"
    | "derevnya"
    | "hutor"
    | "pogost"
    | "stanitsa"
    | "mestechko";

  export const ADMIN_DIVISION_TYPE_LABELS: Record<AdminDivisionType, string> = {
    governorate: "Губерния",
    district: "Уезд",
    volost: "Волость",
    other: "Иное",
    gorod: "Город",
    selo: "Село",
    derevnya: "Деревня",
    hutor: "Хутор",
    pogost: "Погост",
    stanitsa: "Станица",
    mestechko: "Местечко",
  };

  export function adminDivisionTypeLabel(t: string): string {
    return ADMIN_DIVISION_TYPE_LABELS[t as AdminDivisionType] ?? t;
  }

  // AdminDivisionInput — тело POST/PUT /api/admin-divisions (transport.AdminDivisionCreate
  // и transport.AdminDivisionUpdate имеют одинаковую форму: полная замена name/type/parent_id).
  export interface AdminDivisionInput {
    name: string;
    type: AdminDivisionType;
    parent_id: string | null;
  }
  ```

- [ ] Шаг 1.5. `web/src/api.ts` — сразу перед `export interface DocFile`
  добавь функции записи и чтения одной единицы:
  ```typescript
  // fetchDivision — GET /api/admin-divisions/{id}, открыто анонимному
  // посетителю (auth.md §6). Через authFetch — ради ApiError (нужен код 404
  // на странице View: единицу могли удалить в другой вкладке).
  export async function fetchDivision(id: string): Promise<AdminDivision> {
    return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`);
  }

  // createDivision/updateDivision/deleteDivision — запись, только для
  // вошедшего владельца (requireFull на сервере, internal/httpapi/division_write.go).
  export async function createDivision(input: AdminDivisionInput): Promise<AdminDivision> {
    return authFetch<AdminDivision>("/api/admin-divisions", {
      method: "POST",
      body: JSON.stringify(input),
    });
  }

  export async function updateDivision(id: string, input: AdminDivisionInput): Promise<AdminDivision> {
    return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  }

  export async function deleteDivision(id: string): Promise<void> {
    return authFetch<void>(`/api/admin-divisions/${encodeURIComponent(id)}`, { method: "DELETE" });
  }

  ```

- [ ] Шаг 1.6. Рубеж: из `web/` — `npm run typecheck` чисто, `npm run
  build` чисто (сборка тоже должна пройти без ошибок — новые экспорты
  пока никем не используются, это нормально до Задачи 2).

- [ ] Шаг 1.7. Коммит:
  `git add web/src/auth.ts web/src/api.ts`
  `feat(web): authFetch наружу + клиент записи административного деления`.

## Задача 2. Страницы: список-дерево, просмотр/редактирование, форма создания

**Интерфейсы, потребляемые из Задачи 1** (уже в `web/src/api.ts` и
`web/src/auth.ts` к моменту этой задачи):
`fetchAdminDivisions`, `searchAdminDivisions`, `fetchDivision`,
`createDivision`, `updateDivision`, `deleteDivision`, `MAX_PAGE_LIMIT`,
`adminDivisionTypeLabel`, `ADMIN_DIVISION_TYPE_LABELS`, тип
`AdminDivision`, тип `AdminDivisionType`, `authFetch`, `ApiError`, тип
`ApiErrorReferrer` — все сигнатуры см. в файлах после Задачи 1.
`useSession()` (`web/src/session.tsx`) отдаёт `{ session: AuthSession |
null, ... }` — `session != null` значит «залогиненный владелец», только
тогда показываются кнопки создания/редактирования/удаления (бэк это же
проверяет через `requireFull`).

**Файлы:**
- Создать: `web/src/pages/DivisionForm.tsx`, `web/src/pages/DivisionsList.tsx`,
  `web/src/pages/DivisionView.tsx`

- [ ] Шаг 2.1. Создай `web/src/pages/DivisionForm.tsx` — общая модалка
  создания единицы (используется и List для корня, и View для дочерней —
  `parentId` фиксирован пропом, не полем формы):
  ```typescript
  import { useState } from "react";
  import { Alert, Form, Input, Modal, Select } from "antd";
  import {
    ADMIN_DIVISION_TYPE_LABELS,
    createDivision,
    type AdminDivision,
    type AdminDivisionType,
  } from "../api";
  import { ApiError } from "../auth";

  export const TYPE_OPTIONS = (Object.keys(ADMIN_DIVISION_TYPE_LABELS) as AdminDivisionType[]).map(
    (value) => ({ value, label: ADMIN_DIVISION_TYPE_LABELS[value] }),
  );

  interface DivisionFormValues {
    name: string;
    type: AdminDivisionType;
  }

  const FORM_FIELDS: (keyof DivisionFormValues)[] = ["name", "type"];

  // CreateDivisionModal — форма создания единицы, используется и на
  // DivisionsList (parentId=null — корень) и на DivisionView (parentId —
  // текущая единица, «добавить дочернюю»). Единица создаётся с фиксированным
  // parent_id из пропа: сам parent_id полем формы не является.
  export function CreateDivisionModal({
    open,
    parentId,
    onClose,
    onCreated,
  }: {
    open: boolean;
    parentId: string | null;
    onClose: () => void;
    onCreated: (created: AdminDivision) => void;
  }) {
    const [form] = Form.useForm<DivisionFormValues>();
    const [error, setError] = useState<string | null>(null);
    const [submitting, setSubmitting] = useState(false);

    const handleClose = () => {
      form.resetFields();
      setError(null);
      onClose();
    };

    const onFinish = async (values: DivisionFormValues) => {
      setSubmitting(true);
      setError(null);
      try {
        const created = await createDivision({
          name: values.name,
          type: values.type,
          parent_id: parentId,
        });
        form.resetFields();
        onCreated(created);
      } catch (e) {
        if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
          form.setFields([{ name: e.field as keyof DivisionFormValues, errors: [e.message] }]);
        } else {
          setError(e instanceof ApiError ? e.message : "Не удалось создать единицу");
        }
      } finally {
        setSubmitting(false);
      }
    };

    return (
      <Modal
        title={parentId == null ? "Добавить в корень" : "Добавить дочернюю единицу"}
        open={open}
        onCancel={handleClose}
        onOk={() => form.submit()}
        okText="Создать"
        cancelText="Отмена"
        confirmLoading={submitting}
        destroyOnHidden
      >
        {error != null && (
          <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />
        )}
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item
            name="name"
            label="Название"
            rules={[{ required: true, message: "Введите название" }]}
          >
            <Input autoFocus />
          </Form.Item>
          <Form.Item name="type" label="Тип" rules={[{ required: true, message: "Выберите тип" }]}>
            <Select options={TYPE_OPTIONS} placeholder="Выберите тип" />
          </Form.Item>
        </Form>
      </Modal>
    );
  }
  ```

- [ ] Шаг 2.2. Создай `web/src/pages/DivisionsList.tsx` — дерево от корня:
  ```typescript
  import { useEffect, useState } from "react";
  import { useNavigate } from "react-router-dom";
  import { Alert, Button, Card, Input, List, Spin, Tree, Typography } from "antd";
  import type { DataNode, EventDataNode } from "antd/es/tree";
  import {
    adminDivisionTypeLabel,
    fetchAdminDivisions,
    searchAdminDivisions,
    MAX_PAGE_LIMIT,
    type AdminDivision,
  } from "../api";
  import { useSession } from "../session";
  import { CreateDivisionModal } from "./DivisionForm";

  function divisionLabel(d: AdminDivision): string {
    return `${d.name} (${adminDivisionTypeLabel(d.type)})`;
  }

  function toTreeNode(d: AdminDivision): DataNode {
    return { key: d.id, title: divisionLabel(d) };
  }

  // updateTreeData — иммутабельно подставляет загруженных детей узла key в
  // дерево antd Tree (рекурсивно, узел может быть на любой глубине).
  function updateTreeData(list: DataNode[], key: string, children: DataNode[]): DataNode[] {
    return list.map((node) => {
      if (node.key === key) {
        return { ...node, children, isLeaf: children.length === 0 };
      }
      if (node.children != null) {
        return { ...node, children: updateTreeData(node.children, key, children) };
      }
      return node;
    });
  }

  // DivisionsList — «Административное деление»: дерево от корня. Без
  // parent_id бэк (internal/usecases/list_divisions/scenario.go:31-33)
  // отдаёт ВЕСЬ список без фильтра по родителю (комментарий
  // "nil — корень" в internal/models/query.go:67 вводит в заблуждение —
  // настоящей фильтрации на корень там нет), поэтому корень фильтруем на
  // клиенте по parent_id === null. Дети подгружаются по клику на
  // раскрывашку — там parent_id передаётся явно, и бэк (listChildren, та
  // же scenario.go) фильтрует по-настоящему. Поиск по названию временно
  // подменяет дерево плоским списком найденного (как SettlementsTab). Клик
  // по названию узла — переход на View (/divisions/:id).
  export default function DivisionsList() {
    const navigate = useNavigate();
    const { session } = useSession();
    const [treeData, setTreeData] = useState<DataNode[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [searchResults, setSearchResults] = useState<AdminDivision[] | null>(null);
    const [searching, setSearching] = useState(false);
    const [createOpen, setCreateOpen] = useState(false);

    const loadRoot = () => {
      setLoading(true);
      setError(null);
      fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
        .then((items) =>
          setTreeData(items.filter((d) => d.parent_id == null).map(toTreeNode)),
        )
        .catch((e: Error) => setError(e.message))
        .finally(() => setLoading(false));
    };

    useEffect(() => {
      loadRoot();
    }, []);

    const onLoadData = async (node: EventDataNode<DataNode>) => {
      const children = await fetchAdminDivisions({
        parent_id: String(node.key),
        limit: MAX_PAGE_LIMIT,
      });
      setTreeData((prev) => updateTreeData(prev, String(node.key), children.map(toTreeNode)));
    };

    const onSelect = (keys: React.Key[]) => {
      if (keys.length > 0) {
        navigate(`/divisions/${keys[0]}`);
      }
    };

    const onSearch = (value: string) => {
      const q = value.trim();
      if (!q) {
        setSearchResults(null);
        return;
      }
      setSearching(true);
      setError(null);
      searchAdminDivisions({ q, limit: MAX_PAGE_LIMIT })
        .then(setSearchResults)
        .catch((e: Error) => setError(e.message))
        .finally(() => setSearching(false));
    };

    const onSearchChange = (value: string) => {
      if (value.trim() === "") {
        setSearchResults(null);
      }
    };

    return (
      <Card
        title="Административное деление"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить в корень
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по названию…"
          allowClear
          enterButton
          loading={searching}
          onSearch={onSearch}
          onChange={(e) => onSearchChange(e.target.value)}
          style={{ marginBottom: 16 }}
        />
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {searchResults != null ? (
          <List
            dataSource={searchResults}
            locale={{ emptyText: "Найдено пусто" }}
            renderItem={(d) => (
              <List.Item>
                <Typography.Link onClick={() => navigate(`/divisions/${d.id}`)}>
                  {divisionLabel(d)}
                </Typography.Link>
              </List.Item>
            )}
          />
        ) : loading ? (
          <Spin />
        ) : (
          <Tree treeData={treeData} loadData={onLoadData} onSelect={onSelect} showLine />
        )}
        <CreateDivisionModal
          open={createOpen}
          parentId={null}
          onClose={() => setCreateOpen(false)}
          onCreated={(d) => {
            setCreateOpen(false);
            loadRoot();
            navigate(`/divisions/${d.id}`);
          }}
        />
      </Card>
    );
  }
  ```

- [ ] Шаг 2.3. Создай `web/src/pages/DivisionView.tsx` — просмотр,
  переключатель на редактирование, дочерние единицы, удаление:
  ```typescript
  import { useEffect, useState } from "react";
  import { Link, useNavigate, useParams } from "react-router-dom";
  import {
    Alert,
    Breadcrumb,
    Button,
    Card,
    Descriptions,
    Form,
    Input,
    List,
    Modal,
    Popconfirm,
    Select,
    Space,
    Spin,
    Typography,
  } from "antd";
  import {
    adminDivisionTypeLabel,
    deleteDivision,
    fetchAdminDivisions,
    fetchDivision,
    searchAdminDivisions,
    updateDivision,
    MAX_PAGE_LIMIT,
    type AdminDivision,
    type AdminDivisionType,
  } from "../api";
  import { ApiError, type ApiErrorReferrer } from "../auth";
  import { useSession } from "../session";
  import { CreateDivisionModal, TYPE_OPTIONS } from "./DivisionForm";

  function divisionLabel(d: AdminDivision): string {
    return `${d.name} (${adminDivisionTypeLabel(d.type)})`;
  }

  interface EditFormValues {
    name: string;
    type: AdminDivisionType;
  }

  const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name", "type"];

  // DivisionView — просмотр единицы, переключаемый в форму редактирования
  // (та же страница, без смены URL — решение: PUT заменяет
  // name/type/parent_id разом, автосейв по полям не подходит). Дочерние
  // единицы — список со ссылками на их View; «добавить дочернюю» открывает
  // CreateDivisionModal с parentId = текущая единица. Удаление — Popconfirm,
  // конфликт (409, единица занята) — модалка со списком ссылающихся.
  export default function DivisionView() {
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const { session } = useSession();

    const [division, setDivision] = useState<AdminDivision | null>(null);
    const [parent, setParent] = useState<AdminDivision | null>(null);
    const [children, setChildren] = useState<AdminDivision[]>([]);
    const [loading, setLoading] = useState(true);
    const [notFound, setNotFound] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const [editing, setEditing] = useState(false);
    const [form] = Form.useForm<EditFormValues>();
    const [saveError, setSaveError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);

    // editParentId — родитель, выбранный в форме редактирования (null — корень).
    const [editParentId, setEditParentId] = useState<string | null>(null);
    const [editParentLabel, setEditParentLabel] = useState<string | null>(null);
    const [parentPickerOpen, setParentPickerOpen] = useState(false);
    const [parentQuery, setParentQuery] = useState("");
    const [parentResults, setParentResults] = useState<AdminDivision[]>([]);
    const [parentSearching, setParentSearching] = useState(false);

    const [addChildOpen, setAddChildOpen] = useState(false);
    const [deleting, setDeleting] = useState(false);
    const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

    const load = (divisionId: string) => {
      setLoading(true);
      setNotFound(false);
      setError(null);
      fetchDivision(divisionId)
        .then((d) => {
          setDivision(d);
          if (d.parent_id != null) {
            fetchDivision(d.parent_id)
              .then(setParent)
              .catch(() => setParent(null));
          } else {
            setParent(null);
          }
          return fetchAdminDivisions({ parent_id: divisionId, limit: MAX_PAGE_LIMIT });
        })
        .then(setChildren)
        .catch((e) => {
          if (e instanceof ApiError && e.status === 404) {
            setNotFound(true);
          } else {
            setError(e instanceof Error ? e.message : "Не удалось загрузить единицу");
          }
        })
        .finally(() => setLoading(false));
    };

    useEffect(() => {
      if (id != null) {
        load(id);
      }
    }, [id]);

    const startEdit = () => {
      if (division == null) {
        return;
      }
      form.setFieldsValue({ name: division.name, type: division.type as AdminDivisionType });
      setEditParentId(division.parent_id);
      setEditParentLabel(parent != null ? divisionLabel(parent) : null);
      setParentPickerOpen(false);
      setSaveError(null);
      setEditing(true);
    };

    const cancelEdit = () => {
      setEditing(false);
      setParentPickerOpen(false);
      setSaveError(null);
    };

    const onParentSearch = (value: string) => {
      setParentQuery(value);
      const q = value.trim();
      if (!q) {
        setParentResults([]);
        return;
      }
      setParentSearching(true);
      searchAdminDivisions({ q, limit: 20 })
        .then((results) => setParentResults(results.filter((r) => r.id !== division?.id)))
        .catch(() => setParentResults([]))
        .finally(() => setParentSearching(false));
    };

    const pickParent = (d: AdminDivision | null) => {
      setEditParentId(d?.id ?? null);
      setEditParentLabel(d != null ? divisionLabel(d) : null);
      setParentPickerOpen(false);
      setParentQuery("");
      setParentResults([]);
    };

    const onSave = async (values: EditFormValues) => {
      if (division == null) {
        return;
      }
      setSaving(true);
      setSaveError(null);
      try {
        const updated = await updateDivision(division.id, {
          name: values.name,
          type: values.type,
          parent_id: editParentId,
        });
        setDivision(updated);
        if (updated.parent_id != null) {
          fetchDivision(updated.parent_id)
            .then(setParent)
            .catch(() => setParent(null));
        } else {
          setParent(null);
        }
        setEditing(false);
      } catch (e) {
        if (e instanceof ApiError && e.field != null && (EDIT_FORM_FIELDS as string[]).includes(e.field)) {
          form.setFields([{ name: e.field as keyof EditFormValues, errors: [e.message] }]);
        } else {
          setSaveError(e instanceof ApiError ? e.message : "Не удалось сохранить изменения");
        }
      } finally {
        setSaving(false);
      }
    };

    const onDelete = async () => {
      if (division == null) {
        return;
      }
      setDeleting(true);
      setError(null);
      try {
        await deleteDivision(division.id);
        navigate(division.parent_id != null ? `/divisions/${division.parent_id}` : "/divisions");
      } catch (e) {
        if (e instanceof ApiError && e.status === 409) {
          setConflict(e.referrers ?? []);
        } else {
          setError(e instanceof ApiError ? e.message : "Не удалось удалить единицу");
        }
      } finally {
        setDeleting(false);
      }
    };

    if (loading) {
      return <Spin />;
    }

    if (notFound) {
      return (
        <Alert
          type="warning"
          showIcon
          message="Единица не найдена"
          description="Возможно, её удалили. Вернитесь к списку."
          action={
            <Link to="/divisions">
              <Button size="small">К списку</Button>
            </Link>
          }
        />
      );
    }

    if (division == null) {
      return error != null ? <Alert type="error" showIcon message={error} /> : null;
    }

    return (
      <Card>
        <Breadcrumb
          style={{ marginBottom: 16 }}
          items={[
            { title: <Link to="/divisions">Административное деление</Link> },
            ...(parent != null
              ? [{ title: <Link to={`/divisions/${parent.id}`}>{parent.name}</Link> }]
              : []),
            { title: division.name },
          ]}
        />
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

        {!editing ? (
          <>
            <Descriptions title={division.name} column={1} bordered size="small">
              <Descriptions.Item label="Тип">{adminDivisionTypeLabel(division.type)}</Descriptions.Item>
              <Descriptions.Item label="Родитель">
                {parent != null ? (
                  <Link to={`/divisions/${parent.id}`}>{parent.name}</Link>
                ) : (
                  <Typography.Text type="secondary">корень</Typography.Text>
                )}
              </Descriptions.Item>
            </Descriptions>
            {session != null && (
              <Space style={{ marginTop: 16 }}>
                <Button onClick={startEdit}>Редактировать</Button>
                <Button onClick={() => setAddChildOpen(true)}>+ добавить дочернюю</Button>
                <Popconfirm
                  title={`Удалить «${division.name}»?`}
                  description="Действие необратимо."
                  okText="Удалить"
                  cancelText="Отмена"
                  onConfirm={onDelete}
                >
                  <Button danger loading={deleting}>
                    Удалить
                  </Button>
                </Popconfirm>
              </Space>
            )}
          </>
        ) : (
          <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 480 }}>
            {saveError != null && (
              <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
            )}
            <Form.Item
              name="name"
              label="Название"
              rules={[{ required: true, message: "Введите название" }]}
            >
              <Input />
            </Form.Item>
            <Form.Item name="type" label="Тип" rules={[{ required: true, message: "Выберите тип" }]}>
              <Select options={TYPE_OPTIONS} />
            </Form.Item>
            <Form.Item label="Родитель">
              <Space direction="vertical" style={{ width: "100%" }}>
                <Space>
                  <Typography.Text>
                    {editParentLabel ?? <Typography.Text type="secondary">корень</Typography.Text>}
                  </Typography.Text>
                  <Button size="small" onClick={() => setParentPickerOpen((v) => !v)}>
                    Изменить
                  </Button>
                  {editParentId != null && (
                    <Button size="small" onClick={() => pickParent(null)}>
                      Сделать корневой
                    </Button>
                  )}
                </Space>
                {parentPickerOpen && (
                  <Card size="small">
                    <Input.Search
                      placeholder="Поиск родителя по названию…"
                      value={parentQuery}
                      onChange={(e) => onParentSearch(e.target.value)}
                      loading={parentSearching}
                      allowClear
                    />
                    <List
                      size="small"
                      dataSource={parentResults}
                      locale={{ emptyText: "Ничего не найдено" }}
                      renderItem={(d) => (
                        <List.Item style={{ cursor: "pointer" }} onClick={() => pickParent(d)}>
                          {divisionLabel(d)}
                        </List.Item>
                      )}
                    />
                  </Card>
                )}
              </Space>
            </Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={saving}>
                Сохранить
              </Button>
              <Button onClick={cancelEdit}>Отмена</Button>
            </Space>
          </Form>
        )}

        <Typography.Title level={5} style={{ marginTop: 24 }}>
          Дочерние единицы
        </Typography.Title>
        <List
          dataSource={children}
          locale={{ emptyText: "Дочерних единиц нет" }}
          renderItem={(c) => (
            <List.Item>
              <Link to={`/divisions/${c.id}`}>{divisionLabel(c)}</Link>
            </List.Item>
          )}
        />

        <CreateDivisionModal
          open={addChildOpen}
          parentId={division.id}
          onClose={() => setAddChildOpen(false)}
          onCreated={() => {
            setAddChildOpen(false);
            fetchAdminDivisions({ parent_id: division.id, limit: MAX_PAGE_LIMIT })
              .then(setChildren)
              .catch(() => {});
          }}
        />

        <Modal
          title="Единица используется"
          open={conflict != null}
          onCancel={() => setConflict(null)}
          footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
        >
          <Typography.Paragraph>
            Нельзя удалить — на единицу ссылаются другие сущности:
          </Typography.Paragraph>
          <List
            size="small"
            dataSource={conflict ?? []}
            renderItem={(r) => (
              <List.Item>
                {r.type} {r.id}
              </List.Item>
            )}
          />
        </Modal>
      </Card>
    );
  }
  ```

- [ ] Шаг 2.4. Рубеж: из `web/` — `npm run typecheck` чисто, `npm run
  build` чисто (эти три файла пока никем не импортируются — это нормально
  до Задачи 3, но синтаксически и типово должны быть чисты сами по себе).

- [ ] Шаг 2.5. Коммит:
  `git add web/src/pages/DivisionForm.tsx web/src/pages/DivisionsList.tsx web/src/pages/DivisionView.tsx`
  `feat(web): страницы административного деления — список-дерево, просмотр/редактирование`.

## Задача 3. Подключение в App.tsx + документация

Выполняется контроллером напрямую (не через subagent-диспетч — маленькая
механическая проводка плюс синхронизация документации, как Задача 3 этапа D
веб-UI).

**Файлы:**
- Изменить: `web/src/App.tsx`, `CHANGELOG.md`, `docs/usage.md` (если там
  перечислены вкладки/страницы веб-UI — проверить по факту)

- [ ] Шаг 3.1. `web/src/App.tsx` — импорты: добавь `useParams` к импорту
  из `"react-router-dom"`, `BankOutlined` к импорту иконок, и:
  ```typescript
  import DivisionsList from "./pages/DivisionsList";
  import DivisionView from "./pages/DivisionView";
  ```
  (после `import DocsPanel from "./docs-panel";`, перед `import LoginPage
  from "./pages/Login";`).

- [ ] Шаг 3.2. `web/src/App.tsx` — перед `export default function App()`
  добавь:
  ```typescript
  // DivisionsTab — «Административное деление»: с id в URL показывает View
  // конкретной единицы, без id — List (дерево от корня). Тот же приём, что
  // DocsPanel использует для docPath — один компонент ветвится по
  // параметру, а не отдельный <Route> на каждый режим (Tabs ниже держит
  // оба под одним маршрутом /divisions/:id?, см. роуты в App()).
  function DivisionsTab() {
    const { id } = useParams<{ id?: string }>();
    return id != null ? <DivisionView /> : <DivisionsList />;
  }

  ```

- [ ] Шаг 3.3. `web/src/App.tsx` — в `<Routes>` внутри `App()`, после
  строки `<Route path="/docs/:docPath*" element={<AppContent />} />`
  добавь:
  ```typescript
          <Route path="/divisions" element={<AppContent />} />
          <Route path="/divisions/:id" element={<AppContent />} />
  ```

- [ ] Шаг 3.4. `web/src/App.tsx` — в массиве `items` компонента
  `AppContent`, между объектом с `key: "settlements"` и объектом с
  `key: "docs"`, добавь:
  ```typescript
              {
                key: "divisions",
                label: (
                  <>
                    <BankOutlined /> Административное деление
                  </>
                ),
                children: <DivisionsTab />,
              },
  ```

- [ ] Шаг 3.5. Рубеж: из `web/` — `npm run typecheck`, `npm run build`
  чисты. Живая проверка в браузере (тот же сценарий, что в разделе
  «Предпосылка»): регистрация/логин владельца, вкладка «Административное
  деление» открывается, создание корневой единицы, дочерней единицы,
  переход по дереву и по хлебным крошкам, редактирование названия с
  сохранением, удаление с редиректом.

- [ ] Шаг 3.6. `CHANGELOG.md` — добавь в `## [Unreleased]` → `### Added`
  пункт про веб-CRUD делений (по образцу уже существующих пунктов о
  делении и об auth).

- [ ] Шаг 3.7. Проверь `docs/usage.md` на упоминание страниц веб-UI
  (`/login`, `/register`, `/settings` там перечислены при описании флага
  `-trust-proxy` и bootstrap-режима) — если формат подразумевает список
  страниц, добавь `/divisions`; если нет — пропусти без правки (не
  придумывать структуру, которой там нет).

- [ ] Шаг 3.8. Коммит:
  `git add web/src/App.tsx CHANGELOG.md docs/usage.md`
  `feat(web): подключить административное деление во вкладки`
  (если `docs/usage.md` не менялся — не добавлять его в `git add`).

## Рубеж прохода

- Владелец может создать/просмотреть/отредактировать/удалить любую единицу
  иерархии административного деления через `/divisions`.
- Анонимному посетителю доступны только List/View, без кнопок записи.
- `npm run typecheck` и `npm run build` зелёные.
- `go build ./...`, `go vet ./...`, `go test ./...` зелёные (бэк не
  менялся этим планом — контрольный прогон, не должно быть регрессий).

## Коммиты

1. `feat(web): authFetch наружу + клиент записи административного деления`
2. `feat(web): страницы административного деления — список-дерево, просмотр/редактирование`
3. `feat(web): подключить административное деление во вкладки`
