import { useEffect, useState, useCallback, useRef } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Alert, Breadcrumb, List, Spin, Typography } from "antd";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { fetchDoc, fetchDocList, type DocFile } from "./api";
import "./docs-panel.css";

export default function DocsPanel() {
  const { docPath } = useParams<{ docPath: string }>();
  const navigate = useNavigate();

  const [files, setFiles] = useState<DocFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [content, setContent] = useState<string | null>(null);
  const [contentLoading, setContentLoading] = useState(false);
  const filesRef = useRef<DocFile[]>([]);

  useEffect(() => {
    fetchDocList()
      .then(setFiles)
      .catch((e: Error) => {
        setError(e.message);
      })
      .finally(() => setLoading(false));
  }, []);
  filesRef.current = files;

  // При монтировании или изменении docPath — загружаем документ
  const [currentFile, setCurrentFile] = useState<DocFile | null>(null);

  useEffect(() => {
    const path = docPath || "index.md";
    setContentLoading(true);
    setContent(null);
    setCurrentFile(null);
    setError(null);
    fetchDoc(path)
      .then((text) => {
        setContent(text);
        setCurrentFile(filesRef.current.find((f) => f.path === path) ?? null);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setContentLoading(false));
  }, [docPath]);

  const openDoc = useCallback(
    (file: DocFile) => {
      navigate(`/docs/${file.path}`, { relative: "path" });
    },
    [navigate],
  );

  const backToList = () => {
    navigate("/docs");
  };

  // Ссылки внутри markdown: относительная ссылка на .md открывает статью на месте.
  // Сначала ищем в корневом списке файлов, если не нашли — пробуем загрузить напрямую.
  const resolveDocLink = async (href: string): Promise<DocFile | null> => {
    if (!href.startsWith("http://") && !href.startsWith("https://") && !href.startsWith("#")) {
      const cleaned = href.replace(/^\.\//, "").replace(/[?#].*$/, "");
      if (cleaned.endsWith(".md")) {
        // Сначала ищем в списке файлов (корневой уровень)
        const found = filesRef.current.find((f) => f.path === cleaned);
        if (found) {
          return found;
        }
        // Если не нашли — пробуем загрузить по полному пути, чтобы проверить существование
        // и создать метаданные "на лету"
        try {
          const title = cleaned.replace(/\.md$/, "");
          await fetchDoc(cleaned);
          return { path: cleaned, title };
        } catch {
          return null;
        }
      }
    }
    return null;
  };

  const markdownComponents = {
    a: ({ href, children }: { href?: string; children?: React.ReactNode }) => {
      const [resolved, setResolved] = useState<DocFile | null>(null);
      const [loading, setLoading] = useState(false);

      useEffect(() => {
        let cancelled = false;
        setLoading(true);
        resolveDocLink(href ?? "")
          .then((r) => {
            if (!cancelled) {
              setResolved(r);
              setLoading(false);
            }
          })
          .catch(() => {
            if (!cancelled) {
              setLoading(false);
            }
          });
        return () => {
          cancelled = true;
        };
      }, [href]);

      if (loading) {
        return <span className="docs-link-loading">{children}</span>;
      }

      if (resolved) {
        return (
          <a
            href="#"
            onClick={(e) => {
              e.preventDefault();
              openDoc(resolved);
            }}
          >
            {children}
          </a>
        );
      }
      return <a href={href}>{children}</a>;
    },
  };

  if (loading) {
    return <Spin />;
  }

  if (currentFile != null) {
    return (
      <>
        <Breadcrumb
          items={[
            { title: <Link to="/">Данные</Link> },
            { title: <a onClick={backToList}>Документация</a> },
            { title: currentFile.title },
          ]}
        />
        {error != null && <Alert type="error" showIcon message={error} />}
        {contentLoading && <Spin style={{ marginTop: 16 }} />}
        {content != null && (
          <article className="docs-article" style={{ maxWidth: 820, lineHeight: 1.7 }}>
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
              {content}
            </ReactMarkdown>
          </article>
        )}
      </>
    );
  }

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Данные</Link> }, { title: "Документация" }]}
      />
      {error != null && <Alert type="error" showIcon message={error} />}
      {!loading && error == null && (
        <List
          dataSource={files}
          locale={{ emptyText: "Документации пока нет" }}
          renderItem={(f) => (
            <List.Item
              style={{ cursor: "pointer", paddingInline: 0 }}
              onClick={() => openDoc(f)}
            >
              <Typography.Text strong>{f.title}</Typography.Text>
              <Typography.Text type="secondary" style={{ marginLeft: 12 }}>
                {f.path}
              </Typography.Text>
            </List.Item>
          )}
        />
      )}
    </>
  );
}
