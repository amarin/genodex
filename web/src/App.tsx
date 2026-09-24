import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import type { ReactNode } from "react";
import { Layout } from "antd";
import { SessionProvider } from "./session";
import { AppHeader } from "./AppHeader";
import DocsPanel from "./docs-panel";
import EntityCatalog from "./pages/EntityCatalog";
import DivisionsList from "./pages/DivisionsList";
import DivisionView from "./pages/DivisionView";
import SurnamesList from "./pages/SurnamesList";
import SurnameView from "./pages/SurnameView";
import PatronymicsList from "./pages/PatronymicsList";
import PatronymicView from "./pages/PatronymicView";
import EstatesList from "./pages/EstatesList";
import EstateView from "./pages/EstateView";
import TitlesList from "./pages/TitlesList";
import TitleView from "./pages/TitleView";
import GivenNamesList from "./pages/GivenNamesList";
import GivenNameView from "./pages/GivenNameView";
import RepositoriesList from "./pages/RepositoriesList";
import RepositoryView from "./pages/RepositoryView";
import ChurchesList from "./pages/ChurchesList";
import ChurchView from "./pages/ChurchView";
import ParishesList from "./pages/ParishesList";
import ParishView from "./pages/ParishView";
import ArchivesList from "./pages/ArchivesList";
import ArchiveView from "./pages/ArchiveView";
import ArchiveNodesList from "./pages/ArchiveNodesList";
import ArchiveNodeView from "./pages/ArchiveNodeView";
import ArchiveDocumentsList from "./pages/ArchiveDocumentsList";
import ArchiveDocumentView from "./pages/ArchiveDocumentView";
import NotesList from "./pages/NotesList";
import NoteView from "./pages/NoteView";
import AttachmentsList from "./pages/AttachmentsList";
import AttachmentView from "./pages/AttachmentView";
import SourcesList from "./pages/SourcesList";
import SourceView from "./pages/SourceView";
import CitationsList from "./pages/CitationsList";
import CitationView from "./pages/CitationView";
import FamiliesList from "./pages/FamiliesList";
import FamilyView from "./pages/FamilyView";
import PeopleList from "./pages/PeopleList";
import PersonView from "./pages/PersonView";
import RelationsList from "./pages/RelationsList";
import RelationView from "./pages/RelationView";
import ResidencesList from "./pages/ResidencesList";
import ResidenceView from "./pages/ResidenceView";
import EventsList from "./pages/EventsList";
import EventView from "./pages/EventView";
import ReferenceCatalog from "./pages/ReferenceCatalog";
import DivisionTypesPage from "./pages/DivisionTypesPage";
import LoginPage from "./pages/Login";
import RegisterPage from "./pages/Register";
import SettingsPage from "./pages/Settings";

const { Content } = Layout;

// PageLayout — общая рамка (шапка + отступы) для каждой страницы каталога.
// Раньше все страницы делили один AppContent с Tabs; теперь у каждой —
// собственный роут, а переключение между ними — через каталог сущностей
// (/) + хлебные крошки на каждой странице, не вкладки.
function PageLayout({ children }: { children: ReactNode }) {
  return (
    <Layout style={{ minHeight: "100vh" }}>
      <AppHeader />
      <Content style={{ padding: 24 }}>{children}</Content>
    </Layout>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Routes>
          <Route path="/" element={<PageLayout><EntityCatalog /></PageLayout>} />
          <Route path="/reference" element={<PageLayout><ReferenceCatalog /></PageLayout>} />
          <Route path="/reference/division-types" element={<PageLayout><DivisionTypesPage /></PageLayout>} />
          <Route path="/docs" element={<PageLayout><DocsPanel /></PageLayout>} />
          <Route path="/docs/:docPath*" element={<PageLayout><DocsPanel /></PageLayout>} />
          <Route path="/divisions" element={<PageLayout><DivisionsList /></PageLayout>} />
          <Route path="/divisions/:id" element={<PageLayout><DivisionView /></PageLayout>} />
          <Route path="/surnames" element={<PageLayout><SurnamesList /></PageLayout>} />
          <Route path="/surnames/:id" element={<PageLayout><SurnameView /></PageLayout>} />
          <Route path="/patronymics" element={<PageLayout><PatronymicsList /></PageLayout>} />
          <Route path="/patronymics/:id" element={<PageLayout><PatronymicView /></PageLayout>} />
          <Route path="/estates" element={<PageLayout><EstatesList /></PageLayout>} />
          <Route path="/estates/:id" element={<PageLayout><EstateView /></PageLayout>} />
          <Route path="/titles" element={<PageLayout><TitlesList /></PageLayout>} />
          <Route path="/titles/:id" element={<PageLayout><TitleView /></PageLayout>} />
          <Route path="/given-names" element={<PageLayout><GivenNamesList /></PageLayout>} />
          <Route path="/given-names/:id" element={<PageLayout><GivenNameView /></PageLayout>} />
          <Route path="/repositories" element={<PageLayout><RepositoriesList /></PageLayout>} />
          <Route path="/repositories/:id" element={<PageLayout><RepositoryView /></PageLayout>} />
          <Route path="/churches" element={<PageLayout><ChurchesList /></PageLayout>} />
          <Route path="/churches/:id" element={<PageLayout><ChurchView /></PageLayout>} />
          <Route path="/parishes" element={<PageLayout><ParishesList /></PageLayout>} />
          <Route path="/parishes/:id" element={<PageLayout><ParishView /></PageLayout>} />
          <Route path="/archives" element={<PageLayout><ArchivesList /></PageLayout>} />
          <Route path="/archives/:id" element={<PageLayout><ArchiveView /></PageLayout>} />
          <Route path="/archive-nodes" element={<PageLayout><ArchiveNodesList /></PageLayout>} />
          <Route path="/archive-nodes/:id" element={<PageLayout><ArchiveNodeView /></PageLayout>} />
          <Route path="/archive-documents" element={<PageLayout><ArchiveDocumentsList /></PageLayout>} />
          <Route path="/archive-documents/:id" element={<PageLayout><ArchiveDocumentView /></PageLayout>} />
          <Route path="/notes" element={<PageLayout><NotesList /></PageLayout>} />
          <Route path="/notes/:id" element={<PageLayout><NoteView /></PageLayout>} />
          <Route path="/attachments" element={<PageLayout><AttachmentsList /></PageLayout>} />
          <Route path="/attachments/:id" element={<PageLayout><AttachmentView /></PageLayout>} />
          <Route path="/sources" element={<PageLayout><SourcesList /></PageLayout>} />
          <Route path="/sources/:id" element={<PageLayout><SourceView /></PageLayout>} />
          <Route path="/citations" element={<PageLayout><CitationsList /></PageLayout>} />
          <Route path="/citations/:id" element={<PageLayout><CitationView /></PageLayout>} />
          <Route path="/families" element={<PageLayout><FamiliesList /></PageLayout>} />
          <Route path="/families/:id" element={<PageLayout><FamilyView /></PageLayout>} />
          <Route path="/people" element={<PageLayout><PeopleList /></PageLayout>} />
          <Route path="/people/:id" element={<PageLayout><PersonView /></PageLayout>} />
          <Route path="/relations" element={<PageLayout><RelationsList /></PageLayout>} />
          <Route path="/relations/:id" element={<PageLayout><RelationView /></PageLayout>} />
          <Route path="/residences" element={<PageLayout><ResidencesList /></PageLayout>} />
          <Route path="/residences/:id" element={<PageLayout><ResidenceView /></PageLayout>} />
          <Route path="/events" element={<PageLayout><EventsList /></PageLayout>} />
          <Route path="/events/:id" element={<PageLayout><EventView /></PageLayout>} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </SessionProvider>
    </BrowserRouter>
  );
}
