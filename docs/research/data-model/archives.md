# Архивная цепочка

## Archive

`id`, `name`, `notes`.

## Fund

`id`, `code`, `archive_id` (строгая), `notes`.

## Inventory (опись)

`id`, `number`, `fund_id` (строгая), `notes`.

## Case (дело)

`id`, `number`, `inventory_id` (строгая), `title?`, `since?`, `until?`, `parish (TextRef?)`,
`settlement (TextRef?)`, `notes []TextRef`, `sources`.

- `since/until` — период (метрические книги за годы).
- Сканы страниц — `Attachment` с `case_id`; ссылка на источник указана через `anchor`.
- `parish/settlement` — предметный охват дела (метрики прихода, нас. пункты), ссылки
  могут вести на ещё не созданные сущности.