package store

import (
	"context"
	"time"
)

// Справочники и данные, читаемые из БД CRM.

type RefItem struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Extra *string `json:"extra"`
}

func (s *Store) refQuery(ctx context.Context, sql string, args ...any) ([]RefItem, error) {
	rows, err := s.Pool.Query(ctx, s.q(sql), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []RefItem{}
	for rows.Next() {
		var r RefItem
		if err := rows.Scan(&r.ID, &r.Name, &r.Extra); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

// SearchUsers ищет сотрудников по имени/логину или по списку id.
func (s *Store) SearchUsers(ctx context.Context, q string, ids []string) ([]RefItem, error) {
	if len(ids) > 0 {
		return s.refQuery(ctx, `SELECT id::text, name, username FROM {crm}.ref_user WHERE id::text = ANY($1)`, ids)
	}
	return s.refQuery(ctx, `SELECT id::text, name, username FROM {crm}.ref_user
		WHERE status AND ($1 = '' OR name ILIKE '%'||$1||'%' OR username ILIKE '%'||$1||'%')
		ORDER BY name LIMIT 30`, q)
}

// SearchContractors ищет контрагентов. kind: juristic (ЮЛ+ИП), physical (физлица/КЛ), "" — все.
func (s *Store) SearchContractors(ctx context.Context, q, kind string, ids []string) ([]RefItem, error) {
	name := cname("c")
	if len(ids) > 0 {
		return s.refQuery(ctx, `SELECT c.id::text, `+name+`, c.inn FROM {crm}.ref_contractor c WHERE c.id::text = ANY($1)`, ids)
	}
	types := []int{1, 2, 3}
	switch kind {
	case "juristic":
		types = []int{1, 3}
	case "physical":
		types = []int{2}
	}
	return s.refQuery(ctx, `SELECT c.id::text, `+name+` AS n, c.inn FROM {crm}.ref_contractor c
		WHERE c.contractor_type_id = ANY($2)
		  AND ($1 = '' OR `+name+` ILIKE '%'||$1||'%' OR c.inn LIKE $1||'%')
		ORDER BY n NULLS LAST LIMIT 30`, q, types)
}

func (s *Store) UserExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, s.q(`SELECT EXISTS(SELECT 1 FROM {crm}.ref_user WHERE id::text=$1)`), id).Scan(&ok)
	return ok, err
}

func (s *Store) ContractorExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, s.q(`SELECT EXISTS(SELECT 1 FROM {crm}.ref_contractor WHERE id::text=$1)`), id).Scan(&ok)
	return ok, err
}

// RelatedTask — задача по тому же контрагенту (из нового сервиса или из CRM).
type RelatedTask struct {
	ID             string     `json:"id"`
	Number         int64      `json:"number"`
	Origin         string     `json:"origin"` // tasks | crm
	TypeName       string     `json:"type_name"`
	Title          string     `json:"title"`
	StatusName     *string    `json:"status_name"`
	ExecutorName   *string    `json:"executor_name"`
	ContractorName *string    `json:"contractor_name"`
	ContactName    *string    `json:"contact_contractor_name"`
	CreatedAt      *time.Time `json:"created_at"`
}

// ContractorTasks возвращает задачи нового сервиса и старые задачи CRM (doc_task) по контрагенту или КЛ.
func (s *Store) ContractorTasks(ctx context.Context, contractorID, contactID *string, excludeID string) ([]RelatedTask, error) {
	if contractorID == nil && contactID == nil {
		return []RelatedTask{}, nil
	}
	rows, err := s.Pool.Query(ctx, s.q(`
		SELECT * FROM (
			SELECT t.id::text, t.number, 'tasks' AS origin, tt.name, t.title, st.name, u.name, `+cname("c")+`, `+cname("cc")+`, t.created_at
			FROM tasks.task t
			JOIN tasks.task_type tt ON tt.id = t.task_type_id
			JOIN tasks.task_status st ON st.id = t.status_id
			LEFT JOIN {crm}.ref_user u ON u.id = t.executor_user_id
			LEFT JOIN {crm}.ref_contractor c ON c.id = t.contractor_id
			LEFT JOIN {crm}.ref_contractor cc ON cc.id = t.contact_contractor_id
			WHERE t.id::text <> $3 AND (t.contractor_id = $1 OR t.contact_contractor_id = $2)
			UNION ALL
			SELECT d.id::text, d.number, 'crm', e.name, COALESCE(d.task_description, ''), st.name, u.name, `+cname("c")+`, `+cname("cc")+`, d.created_at
			FROM {crm}.doc_task d
			JOIN {crm}.enum_entity e ON e.id = d.entity_type_id
			LEFT JOIN {crm}.enum_task_status st ON st.id = d.task_status_id
			LEFT JOIN {crm}.ref_user u ON u.id = d.executor_user_id
			LEFT JOIN {crm}.ref_contractor c ON c.id = d.contractor_id
			LEFT JOIN {crm}.ref_contractor cc ON cc.id = d.contact_contractor_id
			WHERE d.contractor_id = $1 OR d.contact_contractor_id = $2
		) x ORDER BY created_at DESC NULLS LAST LIMIT 50`), contractorID, contactID, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []RelatedTask{}
	for rows.Next() {
		var r RelatedTask
		if err := rows.Scan(&r.ID, &r.Number, &r.Origin, &r.TypeName, &r.Title, &r.StatusName, &r.ExecutorName,
			&r.ContractorName, &r.ContactName, &r.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

// Activity — событие в ленте активности клиента.
type Activity struct {
	Kind   string    `json:"kind"` // call | order | task | crm_task
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	Title  string    `json:"title"`
	Detail *string   `json:"detail"`
}

// ClientActivity собирает звонки, заказы и задачи клиента из CRM и нового сервиса.
func (s *Store) ClientActivity(ctx context.Context, contractorID, contactID *string) ([]Activity, error) {
	if contractorID == nil && contactID == nil {
		return []Activity{}, nil
	}
	rows, err := s.Pool.Query(ctx, s.q(`
		SELECT * FROM (
			SELECT 'call' AS kind, c.id::text, c.date_time AS at,
				CASE c.call_direction_id WHEN 1 THEN 'Входящий звонок' WHEN 2 THEN 'Исходящий звонок' ELSE 'Звонок' END,
				concat_ws(' · ', NULLIF(c.phone_external, ''), CASE WHEN c.talk_time > 0 THEN 'разговор ' || c.talk_time || ' сек.' ELSE 'без разговора' END)
			FROM {crm}.doc_call c
			WHERE c.contractor_id = $1 OR c.contact_contractor_id = $2
			UNION ALL
			SELECT 'order', o.id::text, COALESCE(o.date, o.created_at),
				'Заказ №' || COALESCE(o.order_number, o.number::text),
				CASE WHEN o.order_amount IS NOT NULL THEN 'сумма ' || o.order_amount::text END
			FROM {crm}.doc_order o
			WHERE o.contractor_id = $1 OR o.contractor_id = $2
			UNION ALL
			SELECT 'crm_task', d.id::text, d.created_at, e.name || ' №' || d.number, d.task_description
			FROM {crm}.doc_task d JOIN {crm}.enum_entity e ON e.id = d.entity_type_id
			WHERE d.contractor_id = $1 OR d.contact_contractor_id = $2
			UNION ALL
			SELECT 'task', t.id::text, t.created_at, tt.name || ' №' || t.number, NULLIF(t.title, '')
			FROM tasks.task t JOIN tasks.task_type tt ON tt.id = t.task_type_id
			WHERE t.contractor_id = $1 OR t.contact_contractor_id = $2
		) x WHERE at IS NOT NULL ORDER BY at DESC LIMIT 100`), contractorID, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.Kind, &a.ID, &a.At, &a.Title, &a.Detail); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}
