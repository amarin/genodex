import { Link, useLocation, useNavigate } from "react-router-dom";
import { Button, Layout, Space, Typography } from "antd";
import { logout } from "./auth";
import { useSession } from "./session";

const { Header } = Layout;

// SECTIONS — разделы главного меню после названия сайта. «Данные» — каталог
// сущностей (/), «Справочники» — встроенные в сервер справочники (/reference).
const SECTIONS: { label: string; path: string }[] = [
  { label: "Данные", path: "/" },
  { label: "Справочники", path: "/reference" },
];

// --- общая шапка: используется и /docs, и страницами Login/Register/Settings ---

export function AppHeader() {
  const { loading, session, refresh } = useSession();
  const navigate = useNavigate();
  const { pathname } = useLocation();

  // Активный раздел: /reference/… — «Справочники», всё остальное — «Данные».
  const isActive = (path: string) =>
    path === "/reference" ? pathname.startsWith("/reference") : !pathname.startsWith("/reference");

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
    navigate("/");
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
      <Space size="large">
        <Link to="/" style={{ color: "#fff", fontWeight: 600 }}>
          Генеалогия
        </Link>
        {SECTIONS.map((s) => (
          <Link
            key={s.path}
            to={s.path}
            style={{ color: "#fff", fontSize: 16, opacity: isActive(s.path) ? 1 : 0.75 }}
          >
            {s.label}
          </Link>
        ))}
      </Space>
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
