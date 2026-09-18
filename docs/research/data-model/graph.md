# Карта связей

Сущности, фигурирующие ниже, описаны в разделах
[Люди и идентичность](people.md), [Места](places.md), [Жизненные факты](facts.md),
[Доказательства](evidence.md), [Архивная цепочка](archives.md).

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
Church ◄──0..1── Parish ──► Case (метрики)
Parish/Church ──settlements──► AdministrativeDivision        (TextRef)

Event ──participants──► Person   (с ролью)
Event ──place──► AdministrativeDivision | Church | Parish | текст   (PlaceRef)

Case ──► Inventory ──► Fund ──► Archive
Case ──attachments──► Attachment (сканы)

Source ──anchor──► Attachment | кейс+страница+rect | файл+тайм-код | url
SourceLink: Source ◄── доказательство ──► (Event | Relation | Residence | Person | Family | …)
```

## Строгие vs свободные ссылки

| Вид | Поле |
|---|---|
| **Строгие** (обязаны существовать; графовые/навигационные) | `Relation.person_a/b`, `Residence.person_id/place_id`, `Fund.archive_id`, `Inventory.fund_id`, `Case.inventory_id`, `Source.id` в `SourceLink`, `AdministrativeDivision.parent_id`, участник `Event.participants[].person_id` |
| **Свободные** (`TextRef`: текст или ссылка, цели может не быть) | списки `items`/`members`/`settlements`/`estates`/`titles`, `Church.parish`, `Parish.church`, `Case.parish/settlement`, `Event.place` (`PlaceRef`), примечания `notes` |