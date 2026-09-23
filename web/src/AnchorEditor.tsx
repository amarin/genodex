import { useEffect, useState } from "react";
import { Button, Input, InputNumber, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import { fetchAttachments, type Anchor, type AnchorKind, type Attachment } from "./api";

const KIND_OPTIONS: { value: AnchorKind; label: string }[] = [
  { value: "archive", label: "Архив (узел/документ)" },
  { value: "file", label: "Файл (вложение)" },
  { value: "url", label: "Ссылка" },
];

const EMPTY_ANCHOR: Anchor = { kind: "archive", page: 1 };

// useAttachmentOptions — заполняет Select вложений для anchor.attachment_id.
// В отличие от anchor.node_id/document_id (ArchiveNode/ArchiveDocument ещё
// без CRUD, подпроект 6 — обычные текстовые поля), Attachment уже есть с
// подпроекта 4, поэтому здесь — полноценный searchable Select, по образцу
// useRepositoryOptions/ArchiveForm.tsx (обсуждение подпроекта 5).
function useAttachmentOptions() {
  const [attachments, setAttachments] = useState<Attachment[]>([]);

  useEffect(() => {
    fetchAttachments({ limit: 500 })
      .then(setAttachments)
      .catch(() => setAttachments([]));
  }, []);

  return attachments.map((a) => ({ value: a.id, label: a.filename || a.uri || a.id }));
}

// AnchorEditor — редактор полиморфной привязки «где именно» у Citation
// (models.Anchor): дискриминатор kind (archive/file/url) переключает набор
// полей. Первый полиморфный тип в программе — редактор целиком заменяется
// при смене kind (EMPTY_ANCHOR для нового варианта), а не сохраняет поля
// прежнего варианта.
export function AnchorEditor({
  value,
  onChange,
  addLabel,
}: {
  value: Anchor | null | undefined;
  onChange: (next: Anchor | null) => void;
  addLabel: string;
}) {
  const attachmentOptions = useAttachmentOptions();

  if (value == null) {
    return (
      <Button type="dashed" onClick={() => onChange(EMPTY_ANCHOR)} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    );
  }

  const set = (patch: Partial<Anchor>) => onChange({ ...value, ...patch });

  const setKind = (kind: AnchorKind) => {
    if (kind === "archive") {
      onChange({ kind, page: 1 });
    } else if (kind === "file") {
      onChange({ kind });
    } else {
      onChange({ kind });
    }
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space wrap>
        <Select style={{ width: 220 }} value={value.kind} options={KIND_OPTIONS} onChange={setKind} />
        <MinusCircleOutlined onClick={() => onChange(null)} />
      </Space>
      {value.kind === "archive" && (
        <Space wrap style={{ width: "100%" }}>
          <Input
            placeholder="id архивного узла (AN-…)"
            value={value.node_id}
            onChange={(e) => set({ node_id: e.target.value })}
            style={{ width: 260 }}
          />
          <Input
            placeholder="id архивного документа (DC-…, необязательно)"
            value={value.document_id}
            onChange={(e) => set({ document_id: e.target.value })}
            style={{ width: 300 }}
          />
          <InputNumber
            placeholder="Страница"
            min={1}
            value={value.page}
            onChange={(v) => set({ page: v ?? undefined })}
            style={{ width: 110 }}
          />
          <Input
            placeholder="Область (необязательно)"
            value={value.rect}
            onChange={(e) => set({ rect: e.target.value })}
            style={{ width: 200 }}
          />
        </Space>
      )}
      {value.kind === "file" && (
        <Space wrap style={{ width: "100%" }}>
          <Select
            style={{ width: 300 }}
            placeholder="Вложение"
            options={attachmentOptions}
            value={value.attachment_id || undefined}
            onChange={(v) => set({ attachment_id: v })}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
          <Input
            placeholder="Тайм-код (необязательно)"
            value={value.timecode}
            onChange={(e) => set({ timecode: e.target.value })}
            style={{ width: 160 }}
          />
        </Space>
      )}
      {value.kind === "url" && (
        <Input
          placeholder="https://…"
          value={value.url}
          onChange={(e) => set({ url: e.target.value })}
        />
      )}
    </Space>
  );
}
