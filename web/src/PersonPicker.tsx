import { useEffect, useState } from "react";
import { Select } from "antd";
import { fetchPeople, MAX_PAGE_LIMIT, type Person } from "./api";
import { personDisplayName } from "./PersonNameList";

// usePersonOptions — постранично (до MAX_PAGE_LIMIT — Person уже имеет
// List/Search с подпроекта 8) грузит список персон. Экспортируется отдельно
// от PersonPicker (не только Select-опции), т.к. отображаемое имя по
// person_id нужно и в read-only View-страницах (Relation/Residence/Event) —
// по образцу useRepositoryOptions/ArchiveForm.tsx и повторного fetch'а
// репозиториев в ArchiveView.tsx для подписи, не только для picker'а.
export function usePersonOptions(): Person[] {
  const [people, setPeople] = useState<Person[]>([]);

  useEffect(() => {
    fetchPeople({ limit: MAX_PAGE_LIMIT })
      .then(setPeople)
      .catch(() => setPeople([]));
  }, []);

  return people;
}

// PersonPicker — первый в программе переиспользуемый picker персоны
// (подпроект 9: Relation.PersonA/PersonB — два picker'а на одной форме,
// Residence.PersonID, каждая строка Event.Participants,
// docs/data-model/entity-write.md §3.8). Плоский Select с клиентским
// поиском — тот же принцип "точечный picker под конкретную строгую
// FK-связь", что и useRepositoryOptions/useCitationOptions, но вынесен в
// общий файл, а не в форму одной сущности, т.к. используется в трёх разных
// сущностях этого подпроекта. Никакого дерева/модалки — не нужно, Person
// уже отдаёт плоский список.
export function PersonPicker({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (id: string) => void;
  placeholder?: string;
}) {
  const people = usePersonOptions();
  const options = people.map((p) => ({ value: p.id, label: personDisplayName(p) }));

  return (
    <Select
      style={{ width: 260 }}
      placeholder={placeholder ?? "Персона"}
      options={options}
      value={value || undefined}
      onChange={(v) => onChange(v)}
      showSearch
      filterOption={(input, option) =>
        (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
      }
    />
  );
}
