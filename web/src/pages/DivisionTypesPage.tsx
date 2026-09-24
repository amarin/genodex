import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Alert, Breadcrumb, Card, Table, Tag, Typography } from "antd";
import {
  adminDivisionTypeLabel,
  fetchAdminDivisionTypes,
  type AdminDivisionTypeInfo,
} from "../api";

// DivisionTypesPage — справочник типов административного деления (только
// чтение): код, название, ранг, вид и допустимые дочерние типы. Правила
// задаёт сервер (models.AdminDivisionType.CanContain) и проверяет при
// создании/изменении единиц через HTTP и MCP; порядок строк — канонический,
// как в выпадающих списках.
export default function DivisionTypesPage() {
  const [infos, setInfos] = useState<AdminDivisionTypeInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchAdminDivisionTypes()
      .then(setInfos)
      .catch(() => setError("Не удалось загрузить справочник типов"));
  }, []);

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/reference">Справочники</Link> },
          { title: "Типы административного деления" },
        ]}
      />
      <Card title="Типы административного деления">
        <Typography.Paragraph type="secondary">
          Единицу можно добавить только в единицу с более высоким рангом: в уезд — волость
          или село, но не губернию; в село — только меньший населённый пункт. «Иное» не имеет
          ранга: в него можно добавить любой тип, а само оно допустимо в любой единице
          деления. Правила проверяются сервером — и в веб-интерфейсе, и через MCP.
        </Typography.Paragraph>
        {error != null && <Alert type="error" showIcon message={error} />}
        <Table<AdminDivisionTypeInfo>
          rowKey="type"
          loading={infos == null && error == null}
          dataSource={infos ?? []}
          pagination={false}
          // Поле children — допустимые дочерние типы, а не вложенные строки дерева.
          expandable={{ childrenColumnName: "__no_tree__" }}
          size="small"
          columns={[
            { title: "Тип", dataIndex: "label", width: 170 },
            {
              title: "Код",
              dataIndex: "type",
              width: 170,
              render: (t: string) => <Typography.Text code>{t}</Typography.Text>,
            },
            {
              title: "Ранг",
              dataIndex: "rank",
              width: 70,
              align: "right",
              render: (r: number | null) => r ?? "—",
            },
            {
              title: "Вид",
              dataIndex: "settlement",
              width: 150,
              render: (s: boolean) => (s ? "населённый пункт" : "деление"),
            },
            {
              title: "Можно добавить внутрь",
              dataIndex: "children",
              render: (children: string[]) =>
                children.length === 0 ? (
                  <Typography.Text type="secondary">ничего</Typography.Text>
                ) : (
                  children.map((c) => <Tag key={c}>{adminDivisionTypeLabel(c)}</Tag>)
                ),
            },
          ]}
        />
      </Card>
    </>
  );
}
