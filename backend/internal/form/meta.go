package form

// Справочники для конструктора, отдаются фронтенду через /api/meta.

type FieldTypeDef struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Group string `json:"group"`
	Icon  string `json:"icon"`
}

type SystemFieldDef struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Readonly bool   `json:"readonly"`
}

type BlockKindDef struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

var FieldTypes = []FieldTypeDef{
	{TypeText, "Строка", "Основные", "Edit"},
	{TypeTextarea, "Многострочный текст", "Основные", "Document"},
	{TypeNumber, "Число", "Основные", "Odometer"},
	{TypeMoney, "Сумма", "Основные", "Money"},
	{TypeDate, "Дата", "Основные", "Calendar"},
	{TypeDatetime, "Дата и время", "Основные", "Timer"},
	{TypeCheckbox, "Флажок (да/нет)", "Основные", "Check"},
	{TypeSelect, "Выпадающий список", "Выбор", "ArrowDown"},
	{TypeMultiselect, "Множественный выбор", "Выбор", "Finished"},
	{TypeRadio, "Переключатель", "Выбор", "CircleCheck"},
	{TypePhone, "Телефон", "Контакты", "Phone"},
	{TypeEmail, "E-mail", "Контакты", "Message"},
	{TypeURL, "Ссылка", "Контакты", "Link"},
	{TypeUser, "Сотрудник", "Справочники CRM", "User"},
	{TypeContractor, "Контрагент", "Справочники CRM", "OfficeBuilding"},
	{TypeTable, "Таблица (табличная часть)", "Прочее", "Grid"},
	{TypeInfo, "Информационный текст", "Прочее", "InfoFilled"},
}

var SystemFields = []SystemFieldDef{
	{SysTitle, "Цель задачи", false},
	{SysDescription, "Описание задачи", false},
	{SysResult, "Результат", false},
	{SysExecutor, "Исполнитель", false},
	{SysContractor, "Контрагент", false},
	{SysContact, "Контактное лицо", false},
	{SysSolvePlanAt, "Планируемая дата завершения", false},
	{SysPriority, "Приоритет", false},
	{SysObservers, "Наблюдатели", false},
	{SysStatus, "Статус", true},
	{SysNumber, "Номер", true},
	{SysCreatedAt, "Создано", true},
	{SysSolveFactAt, "Фактическая дата завершения", true},
}

var BlockKinds = []BlockKindDef{
	{BlockFields, "Поля", "Блок с произвольным набором полей"},
	{BlockComments, "Комментарии", "Лента комментариев по задаче"},
	{BlockHistory, "История изменений", "Журнал событий задачи"},
	{BlockContractorTasks, "Другие задачи по контрагенту", "Задачи нового сервиса и CRM по тому же контрагенту/КЛ"},
	{BlockClientActivity, "Активность клиента", "Звонки, заказы и задачи клиента из CRM"},
}

func SystemFieldByKey(key string) (SystemFieldDef, bool) {
	for _, s := range SystemFields {
		if s.Key == key {
			return s, true
		}
	}
	return SystemFieldDef{}, false
}
