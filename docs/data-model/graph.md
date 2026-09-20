# Карта связей

Сущности, фигурирующие ниже, описаны в разделах
[Люди и идентичность](../models/people.md), [Места](../models/places.md), [Жизненные факты](../models/facts.md),
[Доказательства](../models/evidence.md), [Архивная цепочка](../models/archives.md), [Заметки](../models/notes.md).

```
Person ──names──► PersonName                      (вложенный)
Person ──1..*──► Residence ──► AdministrativeDivision
Person ──count*─► Relation ◄──count*── Person     (ребро графа)
Family ──members──► Person | текст                (TextRef)
Person ──estates/titles──► Estate/Title (TextRef)

AdministrativeDivision ──parent──► AdministrativeDivision       (рекурсия)
AdministrativeDivision ──successors──► AdministrativeDivision   (преемственность)
AdministrativeDivision ──renames──► NamedPeriod                  (переименования)
AdministrativeDivision ──items──► текст | узел              (TextRef)
Church ◄──0..1── Parish ──► ArchiveNode (дело) | ArchiveDocument (метрики)
Parish/Church ──settlements──► AdministrativeDivision        (TextRef)

Event ──participants──► Person   (с ролью)
Event ──place──► AdministrativeDivision | Church | Parish | текст   (PlaceRef)

Note ──parent──► Note                                       (книга → главы)

ArchiveNode ──archive──► Archive                            (archive_id)
ArchiveNode ──parent──► ArchiveNode                         (рекурсия)
ArchiveNode(единица учёта) ──documents──► ArchiveDocument   (unit_id)
ArchiveNode | ArchiveDocument ──attachments──► Attachment   (сканы страниц)

Citation ──source──► Source                                  (source_id)
Citation ──anchor──► Attachment | узел/документ+страница+rect | файл+тайм-код | url
Source | Archive ──repository──► Repository                  (repository_id)
SourceLink: Citation ◄── доказательство ──► (Event | Relation | Residence | Person | Family | Note | Repository | …)
```

## Строгие vs свободные ссылки

| Вид | Поле |
|---|---|
| **Строгие** (обязаны существовать; графовые/навигационные) | `Relation.person_a/b`, `Residence.person_id/place_id`, `ArchiveNode.archive_id`, `ArchiveNode.parent_id`, `ArchiveDocument.unit_id`, `Attachment.node_id` (и `document_id`, если задан), `Citation.source_id`, `Note.parent_id` (если задан), `Source.repository_id` (если задан), `Archive.repository_id` (если задан), `Citation.id` в `SourceLink`, `AdministrativeDivision.parent_id`, участник `Event.participants[].person_id` |
| **Свободные** (`TextRef`: текст или ссылка, цели может не быть) | списки `items`/`members`/`settlements`/`estates`/`titles`/`nicknames`/`urls`, `Church.parish`, `Parish.church`, `ArchiveNode.parish/settlements`, `ArchiveDocument.parish/settlements`, `Event.place` (`PlaceRef`), примечания `notes` |