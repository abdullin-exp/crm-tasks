// Package service — бизнес-логика задач, общая для HTTP API и Kafka-консьюмера.
package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"crm-tasks/internal/form"
	"crm-tasks/internal/store"
)

const (
	StatusNew        = 1
	StatusInProgress = 2
	StatusDone       = 3

	SourceManual = "manual"
	SourceKafka  = "kafka"
	SourceAPI    = "api"
)

var ErrDuplicate = errors.New("duplicate")

type Service struct {
	Store *store.Store
}

// TaskInput — входные данные задачи. Для обновления передаётся полное состояние редактируемых полей.
type TaskInput struct {
	TaskTypeID          int            `json:"task_type_id"`
	TypeCode            string         `json:"type_code"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	Priority            int            `json:"priority"`
	ExecutorUserID      *string        `json:"executor_user_id"`
	ContractorID        *string        `json:"contractor_id"`
	ContactContractorID *string        `json:"contact_contractor_id"`
	SolvePlanAt         *time.Time     `json:"solve_plan_at"`
	ResultID            *int           `json:"result_id"`
	Data                map[string]any `json:"data"`
	ExternalID          *string        `json:"external_id"`
	LegacyTaskID        *string        `json:"legacy_task_id"`
	// nil — не менять наблюдателей (например, частичные события Kafka), [] — очистить
	ObserverUserIDs *[]string `json:"observer_user_ids"`
}

func (s *Service) resolveType(ctx context.Context, in TaskInput) (*store.TaskType, error) {
	var (
		tt  *store.TaskType
		err error
	)
	switch {
	case in.TaskTypeID > 0:
		tt, err = s.Store.GetType(ctx, s.Store.Pool, in.TaskTypeID)
	case in.TypeCode != "":
		tt, err = s.Store.GetTypeByCode(ctx, in.TypeCode)
	default:
		return nil, form.Errors{"task_type_id": "не указан тип задачи"}
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, form.Errors{"task_type_id": "тип задачи не найден"}
	}
	return tt, err
}

func emptyToNil(p *string) *string {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}

// validateSystem проверяет системные поля: обязательность по схеме, существование ссылок в CRM, результат.
func (s *Service) validateSystem(ctx context.Context, tt *store.TaskType, in *TaskInput, existing map[string]any) (*store.Result, error) {
	errs := form.Errors{}
	in.ExecutorUserID = emptyToNil(in.ExecutorUserID)
	in.ContractorID = emptyToNil(in.ContractorID)
	in.ContactContractorID = emptyToNil(in.ContactContractorID)
	in.Title = strings.TrimSpace(in.Title)
	if in.SolvePlanAt != nil {
		t := in.SolvePlanAt.Truncate(time.Second)
		in.SolvePlanAt = &t
	}

	if in.ObserverUserIDs != nil {
		seen := map[string]bool{}
		ids := []string{}
		for _, id := range *in.ObserverUserIDs {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			if _, err := uuid.Parse(id); err != nil {
				errs[form.SysObservers] = "некорректный идентификатор наблюдателя"
				break
			}
			seen[id] = true
			ids = append(ids, id)
		}
		in.ObserverUserIDs = &ids
		if len(ids) > 50 {
			errs[form.SysObservers] = "не более 50 наблюдателей"
		} else if len(ids) > 0 && errs[form.SysObservers] == "" {
			n, err := s.Store.CountUsers(ctx, ids)
			if err != nil {
				return nil, err
			}
			if n != len(ids) {
				errs[form.SysObservers] = "наблюдатель не найден в CRM"
			}
		}
	}

	empty := map[string]bool{
		form.SysObservers:   in.ObserverUserIDs == nil || len(*in.ObserverUserIDs) == 0,
		form.SysTitle:       in.Title == "",
		form.SysDescription: strings.TrimSpace(in.Description) == "",
		form.SysExecutor:    in.ExecutorUserID == nil,
		form.SysContractor:  in.ContractorID == nil,
		form.SysContact:     in.ContactContractorID == nil,
		form.SysSolvePlanAt: in.SolvePlanAt == nil,
		form.SysResult:      in.ResultID == nil,
	}
	// значения для условий видимости: данные формы (с учётом текущих) + системные поля
	values := map[string]any{}
	for k, v := range existing {
		values[k] = v
	}
	for k, v := range in.Data {
		values[k] = v
	}
	for k, v := range sysValues(in) {
		values[k] = v
	}
	for key, isEmpty := range empty {
		if f, ok := tt.Form.SystemField(key); ok && f.Required && isEmpty && tt.Form.IsVisible(f, values) {
			errs[key] = fmt.Sprintf("«%s»: обязательное поле", f.Label)
		}
	}

	check := func(key string, id *string, exists func(context.Context, string) (bool, error), msg string) error {
		if id == nil {
			return nil
		}
		if _, err := uuid.Parse(*id); err != nil {
			errs[key] = msg
			return nil
		}
		ok, err := exists(ctx, *id)
		if err != nil {
			return err
		}
		if !ok {
			errs[key] = msg
		}
		return nil
	}
	if err := check(form.SysExecutor, in.ExecutorUserID, s.Store.UserExists, "сотрудник не найден в CRM"); err != nil {
		return nil, err
	}
	if err := check(form.SysContractor, in.ContractorID, s.Store.ContractorExists, "контрагент не найден в CRM"); err != nil {
		return nil, err
	}
	if err := check(form.SysContact, in.ContactContractorID, s.Store.ContractorExists, "контактное лицо не найдено в CRM"); err != nil {
		return nil, err
	}

	if in.Priority == 0 {
		in.Priority = tt.DefaultPriority
	}
	if in.Priority < 1 || in.Priority > 3 {
		errs[form.SysPriority] = "приоритет должен быть от 1 до 3"
	}

	var result *store.Result
	if in.ResultID != nil {
		for i := range tt.Results {
			if tt.Results[i].ID == *in.ResultID {
				result = &tt.Results[i]
			}
		}
		if result == nil {
			errs[form.SysResult] = "результат не относится к типу задачи"
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return result, nil
}

// validate проверяет системные и пользовательские поля и возвращает все ошибки разом.
func (s *Service) validate(ctx context.Context, tt *store.TaskType, in *TaskInput, existing map[string]any) (*store.Result, map[string]any, error) {
	result, err := s.validateSystem(ctx, tt, in, existing)
	var sysErrs form.Errors
	if err != nil && !errors.As(err, &sysErrs) {
		return nil, nil, err
	}
	if in.Data == nil {
		in.Data = map[string]any{}
	}
	data, dataErrs := form.ValidateData(tt.Form, in.Data, existing, sysValues(in))
	if len(sysErrs) > 0 || len(dataErrs) > 0 {
		all := form.Errors{}
		for k, v := range sysErrs {
			all[k] = v
		}
		for k, v := range dataErrs {
			all[k] = v
		}
		return nil, nil, all
	}
	return result, data, nil
}

// sysValues — значения системных полей для вычисления условий видимости.
func sysValues(in *TaskInput) map[string]any {
	v := map[string]any{
		form.SysTitle:       in.Title,
		form.SysDescription: in.Description,
		form.SysPriority:    in.Priority,
	}
	if in.ResultID != nil {
		v[form.SysResult] = *in.ResultID
	}
	if in.ExecutorUserID != nil {
		v[form.SysExecutor] = *in.ExecutorUserID
	}
	if in.ContractorID != nil {
		v[form.SysContractor] = *in.ContractorID
	}
	if in.ContactContractorID != nil {
		v[form.SysContact] = *in.ContactContractorID
	}
	if in.SolvePlanAt != nil {
		v[form.SysSolvePlanAt] = in.SolvePlanAt.Format(time.RFC3339)
	}
	if in.ObserverUserIDs != nil {
		obs := make([]any, len(*in.ObserverUserIDs))
		for i, id := range *in.ObserverUserIDs {
			obs[i] = id
		}
		v[form.SysObservers] = obs
	}
	return v
}

// applyResult вычисляет статус и факт. дату завершения после выбора результата.
func applyResult(row *store.TaskRow, result *store.Result, prevResultID *int) {
	changed := !reflect.DeepEqual(prevResultID, row.ResultID)
	if result == nil {
		if changed && row.StatusID == StatusDone {
			row.StatusID = StatusInProgress
			row.SolveFactAt = nil
		}
		return
	}
	if !changed {
		return
	}
	if result.IsClosing {
		row.StatusID = StatusDone
		now := time.Now().Truncate(time.Second)
		row.SolveFactAt = &now
	} else {
		row.StatusID = StatusInProgress
		row.SolveFactAt = nil
	}
}

// Create создаёт задачу. source — manual | api | kafka.
func (s *Service) Create(ctx context.Context, in TaskInput, source string) (*store.Task, error) {
	tt, err := s.resolveType(ctx, in)
	if err != nil {
		return nil, err
	}
	if !tt.IsActive {
		return nil, form.Errors{"task_type_id": "тип задачи отключён"}
	}
	// Для интеграций цель задачи необязательна — по умолчанию название типа.
	if source != SourceManual && strings.TrimSpace(in.Title) == "" {
		in.Title = tt.Name
	}
	result, data, err := s.validate(ctx, tt, &in, nil)
	if err != nil {
		return nil, err
	}

	if in.Title == "" {
		in.Title = tt.Name
	}
	if in.SolvePlanAt == nil && tt.SolveMinutes != nil {
		t := time.Now().Add(time.Duration(*tt.SolveMinutes) * time.Minute).Truncate(time.Minute)
		in.SolvePlanAt = &t
	}

	row := store.TaskRow{
		ID:                  uuid.NewString(),
		TaskTypeID:          tt.ID,
		FormVersion:         tt.FormVersion,
		StatusID:            StatusNew,
		ResultID:            in.ResultID,
		Priority:            in.Priority,
		Title:               in.Title,
		Description:         in.Description,
		ExecutorUserID:      in.ExecutorUserID,
		ContractorID:        in.ContractorID,
		ContactContractorID: in.ContactContractorID,
		SolvePlanAt:         in.SolvePlanAt,
		Data:                data,
		Source:              source,
		ExternalID:          emptyToNil(in.ExternalID),
		LegacyTaskID:        emptyToNil(in.LegacyTaskID),
	}
	applyResult(&row, result, nil)

	err = s.Store.WithTx(ctx, func(tx pgx.Tx) error {
		inserted, err := s.Store.InsertTask(ctx, tx, row)
		if err != nil {
			return err
		}
		if !inserted {
			return ErrDuplicate
		}
		if in.ObserverUserIDs != nil {
			if err := s.Store.SetObservers(ctx, tx, row.ID, *in.ObserverUserIDs); err != nil {
				return err
			}
		}
		msg := "Задача создана"
		switch source {
		case SourceKafka:
			msg += " из события Kafka"
		case SourceAPI:
			msg += " через API"
		}
		if err := s.Store.AddEvent(ctx, tx, row.ID, "created", msg, map[string]any{"source": source, "external_id": row.ExternalID}); err != nil {
			return err
		}
		return s.Store.AddOutbox(ctx, tx, store.EventTaskCreated, row.ID, map[string]any{"source": source})
	})
	if errors.Is(err, ErrDuplicate) {
		id, ferr := s.Store.FindByExternal(ctx, source, *row.ExternalID)
		if ferr != nil {
			return nil, ferr
		}
		t, ferr := s.Store.GetTask(ctx, s.Store.Pool, id)
		if ferr != nil {
			return nil, ferr
		}
		return t, ErrDuplicate
	}
	if err != nil {
		return nil, err
	}
	return s.Store.GetTask(ctx, s.Store.Pool, row.ID)
}

// Update сохраняет полное состояние редактируемых полей задачи.
func (s *Service) Update(ctx context.Context, id string, in TaskInput) (*store.Task, error) {
	err := s.Store.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.Store.LockTask(ctx, tx, id); err != nil {
			return err
		}
		cur, err := s.Store.GetTask(ctx, tx, id)
		if err != nil {
			return err
		}
		tt, err := s.Store.GetType(ctx, tx, cur.TaskTypeID)
		if err != nil {
			return err
		}
		result, data, err := s.validate(ctx, tt, &in, cur.Data)
		if err != nil {
			return err
		}
		if in.Title == "" {
			in.Title = cur.Title
		}

		row := store.TaskRow{
			ID:                  id,
			FormVersion:         tt.FormVersion,
			StatusID:            cur.StatusID,
			ResultID:            in.ResultID,
			Priority:            in.Priority,
			Title:               in.Title,
			Description:         in.Description,
			ExecutorUserID:      in.ExecutorUserID,
			ContractorID:        in.ContractorID,
			ContactContractorID: in.ContactContractorID,
			SolvePlanAt:         in.SolvePlanAt,
			SolveFactAt:         cur.SolveFactAt,
			Data:                data,
		}
		applyResult(&row, result, cur.ResultID)

		changes := diff(tt, cur, row, result)
		obsChange, err := s.observersChange(ctx, cur, in.ObserverUserIDs)
		if err != nil {
			return err
		}
		if obsChange != nil {
			changes = append(changes, *obsChange)
		}
		if len(changes) == 0 {
			return nil
		}
		if err := s.Store.UpdateTask(ctx, tx, row); err != nil {
			return err
		}
		if obsChange != nil {
			if err := s.Store.SetObservers(ctx, tx, id, *in.ObserverUserIDs); err != nil {
				return err
			}
		}
		labels := make([]string, 0, len(changes))
		for _, c := range changes {
			labels = append(labels, c.Label)
		}
		if err := s.Store.AddEvent(ctx, tx, id, "updated", "Изменено: "+strings.Join(labels, ", "), map[string]any{"changes": changes}); err != nil {
			return err
		}
		evType := store.EventTaskUpdated
		switch {
		case cur.StatusID != StatusDone && row.StatusID == StatusDone:
			evType = store.EventTaskClosed
		case cur.StatusID == StatusDone && row.StatusID != StatusDone:
			evType = store.EventTaskReopened
		}
		return s.Store.AddOutbox(ctx, tx, evType, id, map[string]any{"changes": changes})
	})
	if err != nil {
		return nil, err
	}
	return s.Store.GetTask(ctx, s.Store.Pool, id)
}

// SetStatus — смена статуса (взять в работу / вернуть в работу).
func (s *Service) SetStatus(ctx context.Context, id string, status int) (*store.Task, error) {
	if status != StatusNew && status != StatusInProgress {
		return nil, form.Errors{"status_id": "вручную можно установить только «Новая» или «В работе»; для завершения выберите результат"}
	}
	err := s.Store.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.Store.LockTask(ctx, tx, id); err != nil {
			return err
		}
		cur, err := s.Store.GetTask(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.StatusID == status {
			return nil
		}
		ct, err := tx.Exec(ctx, `UPDATE tasks.task SET status_id=$2::smallint, result_id=CASE WHEN $2::smallint=1 OR status_id=3 THEN NULL ELSE result_id END,
			solve_fact_at=NULL, updated_at=now() WHERE id=$1`, id, status)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		names := map[int]string{StatusNew: "Новая", StatusInProgress: "В работе"}
		if err := s.Store.AddEvent(ctx, tx, id, "status", fmt.Sprintf("Статус: %s → %s", cur.StatusName, names[status]), nil); err != nil {
			return err
		}
		evType := store.EventTaskTaken
		if cur.StatusID == StatusDone {
			evType = store.EventTaskReopened
		} else if status == StatusNew {
			evType = store.EventTaskUpdated
		}
		return s.Store.AddOutbox(ctx, tx, evType, id, map[string]any{"from_status_id": cur.StatusID, "to_status_id": status})
	})
	if err != nil {
		return nil, err
	}
	return s.Store.GetTask(ctx, s.Store.Pool, id)
}

// observersChange возвращает изменение списка наблюдателей (с именами для истории) или nil.
func (s *Service) observersChange(ctx context.Context, cur *store.Task, next *[]string) (*change, error) {
	if next == nil {
		return nil, nil
	}
	oldSet := map[string]bool{}
	oldNames := []string{}
	for _, o := range cur.Observers {
		oldSet[o.ID] = true
		oldNames = append(oldNames, o.Name)
	}
	same := len(oldSet) == len(*next)
	for _, id := range *next {
		if !oldSet[id] {
			same = false
		}
	}
	if same {
		return nil, nil
	}
	newNames := []string{}
	if len(*next) > 0 {
		users, err := s.Store.SearchUsers(ctx, "", *next)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			newNames = append(newNames, u.Name)
		}
	}
	return &change{form.SysObservers, "Наблюдатели", strings.Join(oldNames, ", "), strings.Join(newNames, ", ")}, nil
}

type change struct {
	Field string `json:"field"`
	Label string `json:"label"`
	Old   any    `json:"old"`
	New   any    `json:"new"`
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func diff(tt *store.TaskType, cur *store.Task, row store.TaskRow, result *store.Result) []change {
	var res []change
	add := func(field string, oldV, newV any) {
		if reflect.DeepEqual(oldV, newV) {
			return
		}
		label := field
		if def, ok := form.SystemFieldByKey(field); ok {
			label = def.Label
		}
		res = append(res, change{field, label, oldV, newV})
	}
	add(form.SysTitle, cur.Title, row.Title)
	add(form.SysDescription, cur.Description, row.Description)
	add(form.SysPriority, cur.Priority, row.Priority)
	add(form.SysExecutor, deref(cur.ExecutorUserID), deref(row.ExecutorUserID))
	add(form.SysContractor, deref(cur.ContractorID), deref(row.ContractorID))
	add(form.SysContact, deref(cur.ContactContractorID), deref(row.ContactContractorID))
	var oldPlan, newPlan any
	if cur.SolvePlanAt != nil {
		oldPlan = cur.SolvePlanAt.UTC().Format(time.RFC3339)
	}
	if row.SolvePlanAt != nil {
		newPlan = row.SolvePlanAt.UTC().Format(time.RFC3339)
	}
	add(form.SysSolvePlanAt, oldPlan, newPlan)
	if !reflect.DeepEqual(cur.ResultID, row.ResultID) {
		var newName any
		if result != nil {
			newName = result.Name
		}
		res = append(res, change{form.SysResult, "Результат", deref(cur.ResultName), newName})
	}
	for _, f := range tt.Form.CustomFields() {
		oldV, newV := cur.Data[f.Code], row.Data[f.Code]
		if !reflect.DeepEqual(oldV, newV) {
			res = append(res, change{f.Code, f.Label, oldV, newV})
		}
	}
	return res
}

// TransferInput — перевод задачи на другого исполнителя.
type TransferInput struct {
	ExecutorUserID string     `json:"executor_user_id"`
	Reason         string     `json:"reason"`
	SolvePlanAt    *time.Time `json:"solve_plan_at"`
}

// checkTransfer проверяет нового исполнителя и причину (общая часть одиночного и массового перевода).
func (s *Service) checkTransfer(ctx context.Context, in *TransferInput) error {
	errs := form.Errors{}
	in.Reason = strings.TrimSpace(in.Reason)
	if _, err := uuid.Parse(in.ExecutorUserID); err != nil {
		errs["executor_user_id"] = "выберите нового исполнителя"
	} else if ok, err := s.Store.UserExists(ctx, in.ExecutorUserID); err != nil {
		return err
	} else if !ok {
		errs["executor_user_id"] = "сотрудник не найден в CRM"
	}
	if in.Reason == "" {
		errs["reason"] = "укажите причину перевода"
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Transfer переводит задачу на другого исполнителя: меняет исполнителя (и срок, если задан),
// задача становится «Новой» для нового исполнителя. Выполненную задачу переводить нельзя.
// Причина пишется в историю и системным комментарием.
func (s *Service) Transfer(ctx context.Context, id string, in TransferInput) (*store.Task, error) {
	if err := s.checkTransfer(ctx, &in); err != nil {
		return nil, err
	}
	if in.SolvePlanAt != nil {
		t := in.SolvePlanAt.Truncate(time.Second)
		in.SolvePlanAt = &t
	}

	err := s.Store.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.Store.LockTask(ctx, tx, id); err != nil {
			return err
		}
		cur, err := s.Store.GetTask(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.StatusID == StatusDone {
			return form.Errors{"status_id": "выполненную задачу нельзя перевести — сначала верните её в работу"}
		}
		if cur.ExecutorUserID != nil && *cur.ExecutorUserID == in.ExecutorUserID {
			return form.Errors{"executor_user_id": "задача уже назначена на этого сотрудника"}
		}
		if _, err := tx.Exec(ctx, `UPDATE tasks.task SET executor_user_id=$2, solve_plan_at=COALESCE($3, solve_plan_at),
			status_id=1, updated_at=now() WHERE id=$1`, id, in.ExecutorUserID, in.SolvePlanAt); err != nil {
			return err
		}
		var newName string
		if err := tx.QueryRow(ctx, s.Store.Q(`SELECT name FROM {crm}.ref_user WHERE id=$1`), in.ExecutorUserID).Scan(&newName); err != nil {
			return err
		}
		oldName := "не назначен"
		if cur.ExecutorName != nil {
			oldName = *cur.ExecutorName
		}
		msg := fmt.Sprintf("Задача переведена: %s → %s. Причина: %s", oldName, newName, in.Reason)
		payload := map[string]any{
			"from_user_id": cur.ExecutorUserID, "to_user_id": in.ExecutorUserID, "reason": in.Reason,
			"changes": []change{{form.SysExecutor, "Исполнитель", oldName, newName}},
		}
		if in.SolvePlanAt != nil {
			var oldPlan any
			if cur.SolvePlanAt != nil {
				oldPlan = cur.SolvePlanAt.Format(time.RFC3339)
			}
			payload["changes"] = append(payload["changes"].([]change), change{form.SysSolvePlanAt, "Планируемая дата завершения", oldPlan, in.SolvePlanAt.Format(time.RFC3339)})
		}
		if err := s.Store.AddEvent(ctx, tx, id, "transfer", msg, payload); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tasks.task_comment (id, task_id, text, is_system) VALUES ($1,$2,$3,true)`,
			uuid.NewString(), id, fmt.Sprintf("Перевод на %s: %s", newName, in.Reason)); err != nil {
			return err
		}
		return s.Store.AddOutbox(ctx, tx, store.EventTaskTransferred, id, map[string]any{
			"from_user_id": cur.ExecutorUserID, "to_user_id": in.ExecutorUserID, "reason": in.Reason,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Store.GetTask(ctx, s.Store.Pool, id)
}

// BulkTransferResult — итог массового перевода.
type BulkTransferResult struct {
	Transferred int               `json:"transferred"`
	Errors      map[string]string `json:"errors"` // id задачи -> причина отказа
}

// BulkTransfer переводит несколько задач. Каждая задача — в своей транзакции:
// ошибка по одной (например, задача уже выполнена) не отменяет перевод остальных.
func (s *Service) BulkTransfer(ctx context.Context, ids []string, in TransferInput) (*BulkTransferResult, error) {
	if len(ids) == 0 {
		return nil, form.Errors{"task_ids": "не выбраны задачи"}
	}
	if len(ids) > 500 {
		return nil, form.Errors{"task_ids": "за один раз можно перевести не более 500 задач"}
	}
	if err := s.checkTransfer(ctx, &in); err != nil {
		return nil, err
	}
	res := &BulkTransferResult{Errors: map[string]string{}}
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			res.Errors[id] = "некорректный id"
			continue
		}
		_, err := s.Transfer(ctx, id, in)
		var verrs form.Errors
		switch {
		case err == nil:
			res.Transferred++
		case errors.As(err, &verrs):
			for _, msg := range verrs {
				res.Errors[id] = msg
			}
		case errors.Is(err, store.ErrNotFound):
			res.Errors[id] = "задача не найдена"
		default:
			return nil, err
		}
	}
	return res, nil
}
