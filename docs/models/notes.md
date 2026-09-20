# Заметки

## Note

`id`, `kind` (`note`/`article`/`book`/`chapter` + открытый), `title?`, `text` (markdown),
`parent_id?` (строгая), `sources []SourceLink`, `private`.

- Слой произвольных текстов: заметка, статья, глава книги, целая книга, выписка из
  парсинга сайта. Текст — markdown.
- `kind=book` + дочерние `kind=chapter` через `parent_id` (строгая ссылка на Note):
  иерархия «книга → главы»; глубина вложенности не ограничена.
- Источник текста — как у любой сущности, через `sources []SourceLink` (например,
  цитата на `Source` kind=`external` с `URLAnchor` для статьи с сайта).
- Не заменяет короткие `notes []TextRef` на сущностях: они остаются. На Note можно
  сослаться через `TextRef` (списки, примечания, `notes`).
- `private` — приватизационный флаг.