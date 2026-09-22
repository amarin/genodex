import { useEffect, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  Layout,
  List,
  Popconfirm,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  ApiError,
  changePassword,
  createAPIToken,
  createInvite,
  listAPITokens,
  revokeAPIToken,
  type APIToken,
} from "../auth";
import { AppHeader, useSession } from "../App";

const { Content } = Layout;

export default function SettingsPage() {
  const { loading, session } = useSession();

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

  if (session == null) {
    return <Navigate to="/login" replace />;
  }

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <AppHeader />
      <Content style={{ padding: 24, maxWidth: 640, margin: "0 auto" }}>
        <Typography.Title level={3}>Настройки</Typography.Title>
        <Space direction="vertical" size="large" style={{ width: "100%" }}>
          <ChangePasswordCard />
          <InviteCard />
          <TokensCard />
        </Space>
      </Content>
    </Layout>
  );
}

interface PasswordFormValues {
  current_password: string;
  new_password: string;
}

function ChangePasswordCard() {
  const { refresh } = useSession();
  const navigate = useNavigate();
  const [form] = Form.useForm<PasswordFormValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onFinish = async (values: PasswordFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      await changePassword(values.current_password, values.new_password);
      // Успех гасит ВСЕ сессии владельца на сервере, включая текущую
      // (internal/httpapi/auth.go, handleAuthPassword) — cookies уже стёрты
      // сервером, здесь только синхронизируем состояние и уходим на /login.
      await refresh();
      navigate("/login");
    } catch (e) {
      if (e instanceof ApiError && e.field != null) {
        form.setFields([{ name: e.field as keyof PasswordFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось сменить пароль");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card title="Смена пароля">
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} style={{ maxWidth: 360 }}>
        <Form.Item
          name="current_password"
          label="Текущий пароль"
          rules={[{ required: true, message: "Введите текущий пароль" }]}
        >
          <Input.Password />
        </Form.Item>
        <Form.Item
          name="new_password"
          label="Новый пароль"
          rules={[{ required: true, message: "Введите новый пароль" }]}
        >
          <Input.Password />
        </Form.Item>
        <Button type="primary" htmlType="submit" loading={submitting}>
          Сменить пароль
        </Button>
      </Form>
    </Card>
  );
}

function InviteCard() {
  const { message } = AntApp.useApp();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [link, setLink] = useState<string | null>(null);

  const onCreate = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const invite = await createInvite();
      setLink(`${window.location.origin}/register?invite=${invite.token}`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Не удалось создать приглашение");
    } finally {
      setSubmitting(false);
    }
  };

  const onCopy = () => {
    if (link == null) {
      return;
    }
    navigator.clipboard
      .writeText(link)
      .then(() => message.success("Ссылка скопирована"))
      .catch(() => message.error("Не удалось скопировать — выделите и скопируйте вручную"));
  };

  return (
    <Card title="Пригласить нового владельца">
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      {link != null && (
        <Alert
          type="success"
          showIcon
          message="Ссылка одноразовая — скопируйте сейчас, второй раз она не показывается"
          description={
            <Space.Compact style={{ width: "100%", marginTop: 8 }}>
              <Input readOnly value={link} onFocus={(e) => e.target.select()} />
              <Button onClick={onCopy}>Скопировать</Button>
            </Space.Compact>
          }
          style={{ marginBottom: 16 }}
        />
      )}
      <Button onClick={onCreate} loading={submitting}>
        Создать ссылку-приглашение
      </Button>
    </Card>
  );
}

function TokensCard() {
  const { message } = AntApp.useApp();
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [label, setLabel] = useState("");
  const [creating, setCreating] = useState(false);
  const [newToken, setNewToken] = useState<{ label: string; token: string } | null>(null);

  const load = () => {
    setLoading(true);
    listAPITokens()
      .then(setTokens)
      .catch((e: Error) => setError(e instanceof ApiError ? e.message : "Не удалось загрузить токены"))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
  }, []);

  const onCreate = async () => {
    if (label.trim() === "") {
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const created = await createAPIToken(label.trim());
      setNewToken({ label: created.label, token: created.token });
      setLabel("");
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Не удалось создать токен");
    } finally {
      setCreating(false);
    }
  };

  const onRevoke = async (id: string) => {
    try {
      await revokeAPIToken(id);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Не удалось отозвать токен");
    }
  };

  const onCopy = (token: string) => {
    navigator.clipboard
      .writeText(token)
      .then(() => message.success("Токен скопирован"))
      .catch(() => message.error("Не удалось скопировать — выделите и скопируйте вручную"));
  };

  return (
    <Card title="API-токены (для MCP)">
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      {newToken != null && (
        <Alert
          type="success"
          showIcon
          message={`Токен «${newToken.label}» создан — скопируйте сейчас, второй раз не показывается`}
          description={
            <Space.Compact style={{ width: "100%", marginTop: 8 }}>
              <Input readOnly value={newToken.token} onFocus={(e) => e.target.select()} />
              <Button onClick={() => onCopy(newToken.token)}>Скопировать</Button>
            </Space.Compact>
          }
          style={{ marginBottom: 16 }}
        />
      )}
      <Space.Compact style={{ marginBottom: 16 }}>
        <Input
          placeholder="Название токена"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          onPressEnter={onCreate}
        />
        <Button onClick={onCreate} loading={creating}>
          Создать токен
        </Button>
      </Space.Compact>
      {loading ? (
        <Spin />
      ) : (
        <List
          dataSource={tokens}
          locale={{ emptyText: "Токенов пока нет" }}
          renderItem={(t) => (
            <List.Item
              actions={
                t.revoked_at == null
                  ? [
                      <Popconfirm
                        key="revoke"
                        title="Отозвать токен?"
                        okText="Отозвать"
                        cancelText="Отмена"
                        onConfirm={() => onRevoke(t.id)}
                      >
                        <Button type="link" danger>
                          Отозвать
                        </Button>
                      </Popconfirm>,
                    ]
                  : []
              }
            >
              <List.Item.Meta
                title={t.label}
                description={
                  t.revoked_at != null
                    ? `Отозван ${new Date(t.revoked_at).toLocaleString("ru")}`
                    : `Создан ${new Date(t.created_at).toLocaleString("ru")}` +
                      (t.last_used_at != null
                        ? `, использован ${new Date(t.last_used_at).toLocaleString("ru")}`
                        : ", ещё не использован")
                }
              />
            </List.Item>
          )}
        />
      )}
    </Card>
  );
}
