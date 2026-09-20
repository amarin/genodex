# Жизненные факты и утверждения

## Event

`id`, `type` (расширяемый, напр. `birth`/`death`/`burial`/`marriage`/`confession`/`census`),
`date FactDate?`, `place PlaceRef?`, `participants [{person_id, role, note?}]`, `sources []SourceLink`,
`notes []TextRef`, `private`.

- Единый тип для всех жизненных фактов: брак и погребение — значения `type`, не отдельные
  сущности.
- У участника есть `role` (для брака — groom/bride, для рождения — subject и т.д.).

## Residence (проживание)

`id`, `person_id` (строгая), `place_id` (строгая, `AdministrativeDivision`),
`since?`, `until?`, `sources []SourceLink`, `note`, `private`.

- Одна персона — несколько проживаний с периодами.