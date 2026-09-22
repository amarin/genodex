import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Alert, Button, Card, Form, Input, Layout } from "antd";
import { login, ApiError } from "../auth";
import { AppHeader } from "../AppHeader";
import { useSession } from "../session";

const { Content } = Layout;

interface LoginFormValues {
  login: string;
  password: string;
}

const LOGIN_FORM_FIELDS: (keyof LoginFormValues)[] = ["login", "password"];

export default function LoginPage() {
  const { refresh } = useSession();
  const navigate = useNavigate();
  const [form] = Form.useForm<LoginFormValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onFinish = async (values: LoginFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      await login(values.login, values.password);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (LOGIN_FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof LoginFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось войти");
      }
      setSubmitting(false);
      return;
    }
    // Вход уже удался на сервере — что бы ни случилось дальше (обновление
    // состояния сессии, переход), это не повод показывать пользователю
    // ошибку входа.
    setSubmitting(false);
    await refresh().catch(() => {});
    navigate("/docs");
  };

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <AppHeader />
      <Content style={{ padding: 24 }}>
        <Card title="Вход" style={{ maxWidth: 400, margin: "48px auto" }}>
          {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
          <Form form={form} layout="vertical" onFinish={onFinish}>
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
                Войти
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </Content>
    </Layout>
  );
}
