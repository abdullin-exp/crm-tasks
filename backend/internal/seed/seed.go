// Package seed создаёт демонстрационные типы задач при первом запуске.
package seed

import (
	"context"
	"log/slog"

	"crm-tasks/internal/form"
	"crm-tasks/internal/store"
)

func ptr[T any](v T) *T { return &v }

func Run(ctx context.Context, st *store.Store) error {
	var n int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM tasks.task_type`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	for _, tt := range []*store.TaskType{emailOrder(), missedCall()} {
		tt.Form.Normalize()
		if err := tt.Form.Validate(); err != nil {
			return err
		}
		if err := st.SaveType(ctx, tt); err != nil {
			return err
		}
	}
	slog.Info("demo task types created")
	return nil
}

func sys(id, key, label string, span int, required bool) form.Field {
	return form.Field{ID: id, Type: form.TypeSystem, System: key, Label: label, Span: span, Required: required}
}

// emailOrder — аналог задачи CRM «Заявка с почты» (TaskEmailOrder), собранный конструктором.
func emailOrder() *store.TaskType {
	return &store.TaskType{
		Code: "email_order", Name: "Заявка с почты", Color: "#67c23a", IsActive: true, DefaultPriority: 2,
		Description:  "Клиент разместил заказ через почту",
		SolveMinutes: ptr(60), LegacyEntityID: nil,
		Results: []store.Result{
			{Name: "Заказ оформлен", Color: "success", IsClosing: true},
			{Name: "Не дозвонился", Color: "warning", IsClosing: false},
			{Name: "Перезвонить позже", Color: "warning", IsClosing: false},
			{Name: "Большие сроки поставки (отказ)", Color: "primary", IsClosing: true},
			{Name: "Нет в ассортименте (отказ)", Color: "primary", IsClosing: true},
			{Name: "Нет в наличии (отказ)", Color: "primary", IsClosing: true},
			{Name: "Высокие цены (отказ)", Color: "primary", IsClosing: true},
			{Name: "Другое (отказ)", Color: "primary", IsClosing: true},
			{Name: "Нецелевое обращение", Color: "info", IsClosing: true},
		},
		Form: form.Schema{Blocks: []form.Block{
			{ID: "main", Title: "Основная информация", Kind: form.BlockFields, Span: 24, Fields: []form.Field{
				sys("f1", form.SysTitle, "Цель задачи", 12, true),
				sys("f2", form.SysResult, "Результат", 12, false),
				sys("f3", form.SysExecutor, "Исполнитель", 12, false),
				{ID: "f4", Code: "client_kind", Type: form.TypeRadio, Label: "Тип клиента", Span: 12, Default: "b2c",
					Options: []form.Option{{Value: "b2c", Label: "B2C"}, {Value: "b2b", Label: "B2B"}}, ShowInList: true},
				sys("f5", form.SysSolvePlanAt, "Планируемая дата завершения", 12, false),
				{ID: "f6", Code: "category1", Type: form.TypeSelect, Label: "Категория 1", Span: 12, Options: []form.Option{
					{Value: "interest", Label: "Интерес к покупке"}, {Value: "claim", Label: "Претензия"}, {Value: "consult", Label: "Консультация"}}},
				sys("f7", form.SysContractor, "Контрагент", 12, false),
				{ID: "f8", Code: "category2", Type: form.TypeSelect, Label: "Категория 2", Span: 12, Options: []form.Option{
					{Value: "purchase", Label: "Покупка"}, {Value: "return", Label: "Возврат"}, {Value: "delivery", Label: "Доставка"}}},
				sys("f9", form.SysContact, "Контактное лицо", 12, false),
				sys("f17", form.SysObservers, "Наблюдатели", 12, false),
				{ID: "f10", Code: "order_number", Type: form.TypeText, Label: "Заказ", Span: 12, Placeholder: "№ заказа", ShowInList: true},
				{ID: "f11", Code: "phone", Type: form.TypePhone, Label: "Контактный телефон", Span: 12},
				{ID: "f12", Code: "order_sum", Type: form.TypeMoney, Label: "Сумма заказа", Span: 12, ShowInList: true},
				{ID: "f13", Code: "email", Type: form.TypeEmail, Label: "Электронная почта", Span: 12},
				{ID: "f14", Code: "refusal_reason", Type: form.TypeTextarea, Label: "Причина отказа", Span: 12},
				sys("f15", form.SysDescription, "Описание задачи", 24, false),
				{ID: "f16", Code: "products", Type: form.TypeTable, Label: "Товарная часть", Span: 24, Columns: []form.Column{
					{Code: "name", Label: "Наименование", Type: "text"},
					{Code: "code", Label: "Код товара", Type: "text"},
					{Code: "qty", Label: "Кол-во", Type: "number"},
					{Code: "accuracy", Label: "Точность, %", Type: "number"},
				}},
			}},
			{ID: "comments", Title: "Комментарии", Kind: form.BlockComments, Span: 12},
			{ID: "activity", Title: "Активность клиента", Kind: form.BlockClientActivity, Span: 12},
			{ID: "other", Title: "Другие задачи по контрагенту и контактному лицу", Kind: form.BlockContractorTasks, Span: 24, Collapsible: true},
			{ID: "history", Title: "История изменений", Kind: form.BlockHistory, Span: 24, Collapsible: true, Collapsed: true},
		}},
	}
}

func missedCall() *store.TaskType {
	return &store.TaskType{
		Code: "missed_call", Name: "Перезвон на пропущенный звонок", Color: "#e6a23c", IsActive: true, DefaultPriority: 3,
		Description: "Клиент не дозвонился, нужно перезвонить", SolveMinutes: ptr(15),
		Results: []store.Result{
			{Name: "Дозвонился, вопрос решён", Color: "success", IsClosing: true},
			{Name: "Не дозвонился", Color: "warning", IsClosing: false},
			{Name: "Нецелевой звонок", Color: "info", IsClosing: true},
		},
		Form: form.Schema{Blocks: []form.Block{
			{ID: "main", Title: "Звонок", Kind: form.BlockFields, Span: 24, Fields: []form.Field{
				sys("f1", form.SysTitle, "Цель задачи", 12, true),
				sys("f2", form.SysResult, "Результат", 12, false),
				{ID: "f3", Code: "phone", Type: form.TypePhone, Label: "Номер телефона", Span: 12, Required: true, ShowInList: true},
				sys("f4", form.SysExecutor, "Исполнитель", 12, false),
				{ID: "f5", Code: "call_at", Type: form.TypeDatetime, Label: "Время звонка", Span: 12},
				sys("f6", form.SysSolvePlanAt, "Перезвонить до", 12, false),
				sys("f7", form.SysContractor, "Контрагент", 12, false),
				{ID: "f8", Code: "attempts", Type: form.TypeNumber, Label: "Попыток дозвона", Span: 12, Min: ptr(0.0)},
			}},
			{ID: "comments", Title: "Комментарии", Kind: form.BlockComments, Span: 24},
			{ID: "history", Title: "История изменений", Kind: form.BlockHistory, Span: 24, Collapsible: true, Collapsed: true},
		}},
	}
}
