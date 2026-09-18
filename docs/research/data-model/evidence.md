# Доказательства

## Source

`id`, `kind` (`archival-scan`/`transcription`/`document`/`audio`/`photo`/`memory`/`external`),
`title`, `anchor Anchor?`, `text?` (расшифровка/текст записи), `author?`, `date?`,
`reliability`, `notes []TextRef`.

- Единая абстракция доказательства:
  1. ссылка на скан страницы дела в архиве (+ область на скане через `anchor.rect`);
  2. текстовая расшифровка (как фрагмент/запись в деле) — `text`;
  3. ссылка на файл другого типа (скан документа, аудио с тайм-меткой, фото события) — `anchor`.
- `reliability` Документа: градация достоверности (например: первичный документ /
  современная запись / воспоминания / косвенные сведения / неизвестно).

## SourceLink

см. [Типы значений](values.md). Применяется к любому утверждению: атрибуты `Person`, `Event`,
`Relation`, `Residence`, `Family`, атрибуты `AdministrativeDivision` и др.
`reliability` здесь — частная: точность именно этого утверждения по этому источнику.

## Attachment

`id`, `kind` (`scan`/`document`/`audio`/`photo`), `uri`/`filename`, `mime`, `page?`,
`case_id?`, `note`.

- Файловый объект; сканы страниц дела привязываются к `Case` через `case_id`,
  источник ссылается на вложение через `anchor.attachment_id`.