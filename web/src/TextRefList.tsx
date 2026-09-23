import { Button, Input, Space, Typography } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { TextRef } from "./api";

// TextRefListEditor — общий редактор списков вроде Surname.variants
// (docs/data-model/entity-write.md §4): v1 редактирует только text —
// добавить/удалить/поменять строку. Элемент с уже заполненным ref
// показывается как текст + пометка ссылки, поле неактивно (picker на
// произвольную сущность — отдельная работа, не в этом проходе).
export function TextRefListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: TextRef[];
  onChange: (next: TextRef[]) => void;
  addLabel: string;
}) {
  const setText = (i: number, text: string) => {
    const next = value.slice();
    next[i] = { ...next[i], text };
    onChange(next);
  };

  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));

  const add = () => onChange([...value, { text: "" }]);

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => {
        const linked = item.ref != null && item.ref !== "";

        return (
          <Space key={i} style={{ width: "100%" }}>
            <Input value={item.text} onChange={(e) => setText(i, e.target.value)} disabled={linked} />
            {linked && (
              <Typography.Text type="secondary">
                → {item.type} {item.ref}
              </Typography.Text>
            )}
            <MinusCircleOutlined onClick={() => remove(i)} />
          </Space>
        );
      })}
      <Button type="dashed" onClick={add} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    </Space>
  );
}
