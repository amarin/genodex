import { useEffect, useState } from "react";
import { Alert, Breadcrumb, List, Spin, Typography } from "antd";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { fetchDoc, fetchDocList, type DocFile } from "./api";
import "./docs-panel.css";

type DocsPanelProps = {
  onError?: (message: string) => void;
};

export default function DocsPanel({ onError }: DocsPanelProps) {
  const [files, setFiles] = useState<DocFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [current, setCurrent] = useState<DocFile | null>(null);
  const [content, setContent] = useState<string | null>(null);
  const [contentLoading, setContentLoading] = useState(false);

  useEffect(() => {
    fetchDocList()
      .then(setFiles)
      .catch((e: Error) => {
        setError(e.message);
        onError?.(e.message);
      })
      .finally(() => setLoading(false));
  }, [onError]);

  const openDoc = (file: DocFile) => {
    setCurrent(file);
    setContentLoading(true);
    setContent(null);
    fetchDoc(file.path)
      .then(setContent)
      .catch((e: Error) => setError(e.message))
      .finally(() => setContentLoading(false));
  };

  const backToList = () => {
    setCurrent(null);
    setContent(null);
    setError(null);
  };

  // Ссылки внутри markdown: относительная ссылка на .md открывает статью на месте.
  const resolveDocLink = (href: string): DocFile | null => {
    if (!current || href.startsWith("http://") || href.startsWith("https://") || href.startsWith("#")) {
      return null;
    }
    const baseDir = current.path.includes("/")
      ? current.path.slice(0, current.path.lastIndexOf("/") + 1)
      : "";
    const raw = `${baseDir}${href}`;
    const cleaned = raw.replace(/^\.\//, "").replace(/[?#].*$/, "");
    if (!cleaned.endsWith(".md")) {
      return null;
    }
    return files.find((f) => f.path === cleaned) ?? null;
  };

  const markdownComponents = {
    a: ({ href, children }: { href?: string; children?: React.ReactNode }) => {
      const target = resolveDocLink(href ?? "");
      if (target) {
        return (
          <a
            href="#"
            onClick={(e) => {
              e.preventDefault();
              openDoc(target);
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

  if (current != null) {
    return (
      <>
        <Breadcrumb
          items={[
            { title: <a onClick={backToList}>Документация</a> },
            { title: current.title },
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