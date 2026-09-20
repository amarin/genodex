# Идентификаторы

- Строковые, глобально уникальные в хранилище; тип `models.ID`.
- Формат: `ПРЕФИКС-ULID`, например `I-01J8X4T0K2M9Q7R5V3B6N8C1D4`. ULID — 26
  символов алфавита Crockford (заглавные, без `I L O U`), время + случайность.
- Префикс кодирует тип сущности; `I F S R N O` совпадают с GEDCOM
  (INDI, FAM, SOUR, REPO, NOTE, OBJE):

  | Префикс | Тип | Префикс | Тип |
  |---|---|---|---|
  | `I` | person | `SN` | surname |
  | `F` | family | `GN` | given_name |
  | `S` | source | `PN` | patronymic |
  | `R` | repository | `ES` | estate |
  | `N` | note | `TT` | title |
  | `O` | attachment | `CH` | church |
  | `E` | event | `PR` | parish |
  | `C` | citation | `AD` | administrative_division |
  | `RL` | relation | `AR` | archive |
  | `RS` | residence | `AN` | archive_node |
  | | | `DC` | archive_document |

- ID генерируют сценарии (интерфейс `IDGenerator`); хранилище ID не создаёт и не
  принимает пустой. Явный ID от клиента допустим только для импорта и проходит
  проверку формата и соответствия префикса типу.
- Читаемость даёт не ID, а `TextRef.text` и словари (`canonical`, `variants`).
- Решение и обоснование — [core-read-write](core-read-write.md) §2.2 и
  [decisions](decisions.md) #30. Прежнее правило (транслит + дисамбигуатор,
  совместимость с vault) отменено.
