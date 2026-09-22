import { Link, useNavigate } from "react-router-dom";
import { Button, Layout, Space, Typography } from "antd";
import { logout } from "./auth";
import { useSession } from "./session";

const { Header } = Layout;

// --- общая шапка: используется и /docs, и страницами Login/Register/Settings ---

export function AppHeader() {
  const { loading, session, refresh } = useSession();
  const navigate = useNavigate();

  const onLogout = async () => {
    try {
      await logout();
    } catch {
      // Не блокируем выход из UI, даже если сеть/сервер подвели — cookies
      // всё равно будут перезаписаны при следующем логине.
    }
    // refresh() тоже может бросить (например resolveAccess на сервере
    // fail-closed'ится при сбое БД) — если не поймать, navigate ниже не
    // выполнится, и шапка продолжит показывать владельца как залогиненного.
    await refresh().catch(() => {});
    navigate("/docs");
  };

  return (
    <Header
      style={{
        color: "#fff",
        fontSize: 18,
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
      }}
    >
      <Link to="/docs" style={{ color: "#fff" }}>
        Genealogy MCP
      </Link>
      {!loading && (
        <Space>
          {session != null ? (
            <>
              <Typography.Text style={{ color: "#fff" }}>{session.login}</Typography.Text>
              <Link to="/settings" style={{ color: "#fff" }}>
                Настройки
              </Link>
              <Button type="link" style={{ color: "#fff" }} onClick={onLogout}>
                Выйти
              </Button>
            </>
          ) : (
            <Link to="/login" style={{ color: "#fff" }}>
              Войти
            </Link>
          )}
        </Space>
      )}
    </Header>
  );
}
