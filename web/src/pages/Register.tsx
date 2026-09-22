import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { Alert, Button, Card, Form, Input, Layout, Spin } from "antd";
import { register, ApiError } from "../auth";
import { AppHeader } from "../AppHeader";
import { useSession } from "../session";

const { Content } = Layout;

interface RegisterFormValues {
  login: string;
  password: string;
}

const REGISTER_FORM_FIELDS: (keyof RegisterFormValues)[] = ["login", "password"];

export default function RegisterPage() {
  const { loading, bootstrap, refresh } = useSession();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const invite = searchParams.get("invite") ?? undefined;
  const [form] = Form.useForm<RegisterFormValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  if (loading) {
    return (
      <Layout style={{ minHeight: "100vh" }}>
        <AppHeader />
        <Content style={{ padding: 24, textAlign: "center" }}>
          <Spin />
        </Content>
      </Layout>
    );
  }

  // auth.md §7: отсутствие обязательного invite, когда bootstrap=false, —
  // ошибка формы ДО отправки, не после попытки submit.
  const missingInvite = !bootstrap && invite == null;

  const onFinish = async (values: RegisterFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      await register(values.login, values.password, invite);
    } catch (e) {
      if (
        e instanceof ApiError &&
        e.field != null &&
        (REGISTER_FORM_FIELDS as string[]).includes(e.field)
      ) {
        form.setFields([{ name: e.field as keyof RegisterFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось зарегистрироваться");
      }
      setSubmitting(false);
      return;
    }
    // Регистрация уже удалась на сервере — переход/обновление сессии не
    // должны выглядеть как провал регистрации.
    setSubmitting(false);
    await refresh().catch(() => {});
    navigate("/docs");
  };

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <AppHeader />
      <Content style={{ padding: 24 }}>
        <Card
          title={bootstrap ? "Регистрация первого владельца" : "Регистрация по приглашению"}
          style={{ maxWidth: 400, margin: "48px auto" }}
        >
          {missingInvite && (
            <Alert
              type="warning"
              showIcon
              message="Нужна ссылка-приглашение от существующего владельца"
              style={{ marginBottom: 16 }}
            />
          )}
          {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
          <Form form={form} layout="vertical" onFinish={onFinish} disabled={missingInvite}>
            <Form.Item name="login" label="Логин" rules={[{ required: true, message: "Введите логин" }]}>
              <Input autoFocus />
            </Form.Item>
            <Form.Item
              name="password"
              label="Пароль"
              rules={[{ required: true, message: "Введите пароль" }]}
            >
              <Input.Password />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" block loading={submitting}>
                Зарегистрироваться
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </Content>
    </Layout>
  );
}
