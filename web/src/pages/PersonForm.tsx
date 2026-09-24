import { useState } from "react";
import { Alert, Checkbox, Form, Modal, Select } from "antd";
import {
  createPerson,
  GENDER_OPTIONS,
  type Person,
  type PersonGender,
  type PersonName,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { PersonNameListEditor } from "../PersonNameList";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface PersonFormValues {
  gender?: PersonGender;
  private?: boolean;
}

const FORM_FIELDS: (keyof PersonFormValues)[] = ["gender"];

// CreatePersonModal — форма создания персоны, структурно по образцу
// CreateFamilyModal (подпроект 7): та же сущность плюс gender (Select) и
// names — новая вложенная подформа-«массив объектов» (PersonNameListEditor,
// docs/data-model/entity-write.md §3.7), вместо простого TextRefListEditor.
// Sources редактируется с рождения контракта (SourceLinkListEditor).
export function CreatePersonModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Person) => void;
}) {
  const [form] = Form.useForm<PersonFormValues>();
  const [names, setNames] = useState<PersonName[]>([]);
  const [estates, setEstates] = useState<TextRef[]>([]);
  const [titles, setTitles] = useState<TextRef[]>([]);
  const [nicknames, setNicknames] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setNames([]);
    setEstates([]);
    setTitles([]);
    setNicknames([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: PersonFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createPerson({
        gender: values.gender ?? "",
        names,
        estates,
        titles,
        nicknames,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof PersonFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить персону"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
      width={720}
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ gender: "", private: false }}>
        <Form.Item name="gender" label="Пол">
          <Select options={GENDER_OPTIONS} />
        </Form.Item>
        <Form.Item label="Имена">
          <PersonNameListEditor value={names} onChange={setNames} addLabel="+ имя" />
        </Form.Item>
        <Form.Item label="Сословия">
          <TextRefListEditor value={estates} onChange={setEstates} addLabel="+ сословие" />
        </Form.Item>
        <Form.Item label="Титулы">
          <TextRefListEditor value={titles} onChange={setTitles} addLabel="+ титул" />
        </Form.Item>
        <Form.Item label="Прозвища">
          <TextRefListEditor value={nicknames} onChange={setNicknames} addLabel="+ прозвище" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
