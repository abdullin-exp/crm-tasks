// Package form описывает схему формы типа задачи (конструктор) и валидацию данных задачи по ней.
package form

import (
	"fmt"
	"regexp"
	"strings"
)

// Типы пользовательских полей
const (
	TypeText        = "text"
	TypeTextarea    = "textarea"
	TypeNumber      = "number"
	TypeMoney       = "money"
	TypeDate        = "date"
	TypeDatetime    = "datetime"
	TypeSelect      = "select"
	TypeMultiselect = "multiselect"
	TypeRadio       = "radio"
	TypeCheckbox    = "checkbox"
	TypePhone       = "phone"
	TypeEmail       = "email"
	TypeURL         = "url"
	TypeUser        = "user"
	TypeContractor  = "contractor"
	TypeTable       = "table"
	TypeInfo        = "info"   // статичный текст, без значения
	TypeSystem      = "system" // привязка к колонке задачи
)

// Системные поля — колонки задачи (аналоги колонок doc_task в CRM)
const (
	SysTitle       = "title"
	SysDescription = "description"
	SysPriority    = "priority"
	SysExecutor    = "executor_user_id"
	SysContractor  = "contractor_id"
	SysContact     = "contact_contractor_id"
	SysSolvePlanAt = "solve_plan_at"
	SysResult      = "result_id"
	SysStatus      = "status_id"
	SysNumber      = "number"
	SysCreatedAt   = "created_at"
	SysSolveFactAt = "solve_fact_at"
	SysObservers   = "observer_user_ids"
)

// Виды блоков
const (
	BlockFields          = "fields"
	BlockComments        = "comments"
	BlockHistory         = "history"
	BlockContractorTasks = "contractor_tasks"
	BlockClientActivity  = "client_activity"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Color string `json:"color,omitempty"`
}

type Column struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Type  string `json:"type"` // text | number | date
}

type Field struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Type        string   `json:"type"`
	System      string   `json:"system,omitempty"`
	Label       string   `json:"label"`
	Required    bool     `json:"required"`
	Readonly    bool     `json:"readonly"`
	Span        int      `json:"span"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []Option `json:"options,omitempty"`
	Columns     []Column `json:"columns,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Text        string   `json:"text,omitempty"`
	ShowInList  bool     `json:"show_in_list"`
	// Для поля типа contractor: juristic | physical | "" (любой)
	ContractorKind string `json:"contractor_kind,omitempty"`
	// Условие показа поля; скрытое поле не проверяется на обязательность
	VisibleIf *Condition `json:"visible_if,omitempty"`
}

type Block struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Kind        string  `json:"kind"`
	Span        int     `json:"span"`
	Collapsible bool    `json:"collapsible"`
	Collapsed   bool    `json:"collapsed"`
	Fields      []Field `json:"fields"`
}

type Schema struct {
	Blocks []Block `json:"blocks"`
}

// IsCustom — поле хранит значение в task.data
func (f Field) IsCustom() bool { return f.Type != TypeSystem && f.Type != TypeInfo }

// CustomFields возвращает все пользовательские поля схемы.
func (s Schema) CustomFields() []Field {
	var res []Field
	for _, b := range s.Blocks {
		if b.Kind != BlockFields {
			continue
		}
		for _, f := range b.Fields {
			if f.IsCustom() {
				res = append(res, f)
			}
		}
	}
	return res
}

// SystemField ищет размещённое на форме системное поле.
func (s Schema) SystemField(key string) (Field, bool) {
	for _, b := range s.Blocks {
		if b.Kind != BlockFields {
			continue
		}
		for _, f := range b.Fields {
			if f.Type == TypeSystem && f.System == key {
				return f, true
			}
		}
	}
	return Field{}, false
}

// Errors — ошибки валидации: ключ (код поля / путь) -> сообщение.
type Errors map[string]string

func (e Errors) Error() string {
	parts := make([]string, 0, len(e))
	for k, v := range e {
		parts = append(parts, k+": "+v)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Normalize проставляет значения по умолчанию (ширины колонок, коды системных полей).
func (s *Schema) Normalize() {
	if s.Blocks == nil {
		s.Blocks = []Block{}
	}
	for bi := range s.Blocks {
		b := &s.Blocks[bi]
		if b.Span <= 0 || b.Span > 24 {
			b.Span = 24
		}
		if b.Kind == "" {
			b.Kind = BlockFields
		}
		if b.Fields == nil {
			b.Fields = []Field{}
		}
		for fi := range b.Fields {
			f := &b.Fields[fi]
			if f.Span <= 0 || f.Span > 24 {
				f.Span = 24
			}
			if f.Type == TypeSystem {
				f.Code = f.System
			}
			if f.Type == TypeInfo && f.Code == "" {
				f.Code = "info_" + f.ID
			}
			if def, ok := SystemFieldByKey(f.System); ok && f.Type == TypeSystem && def.Readonly {
				f.Readonly = true
			}
		}
	}
}

// Validate проверяет корректность схемы при сохранении в конструкторе.
func (s Schema) Validate() error {
	errs := Errors{}
	codes := map[string]bool{}
	blockIDs := map[string]bool{}

	for bi, b := range s.Blocks {
		bp := fmt.Sprintf("blocks[%d]", bi)
		if strings.TrimSpace(b.Title) == "" {
			errs[bp+".title"] = "не задан заголовок блока"
		}
		if !isBlockKind(b.Kind) {
			errs[bp+".kind"] = "неизвестный вид блока: " + b.Kind
		}
		if b.ID == "" || blockIDs[b.ID] {
			errs[bp+".id"] = "пустой или повторяющийся id блока"
		}
		blockIDs[b.ID] = true
		if b.Kind != BlockFields && len(b.Fields) > 0 {
			errs[bp+".fields"] = "поля можно добавлять только в блок вида «Поля»"
		}

		for fi, f := range b.Fields {
			fp := fmt.Sprintf("%s.fields[%d]", bp, fi)
			name := f.Label
			if name == "" {
				name = fp
			}
			if f.Type == TypeSystem {
				if _, ok := SystemFieldByKey(f.System); !ok {
					errs[fp] = "неизвестное системное поле: " + f.System
					continue
				}
			} else if !isFieldType(f.Type) {
				errs[fp] = "неизвестный тип поля: " + f.Type
				continue
			}
			if f.Type != TypeInfo && strings.TrimSpace(f.Label) == "" {
				errs[fp+".label"] = "не задано название поля"
			}
			if f.Type == TypeInfo {
				continue
			}
			if f.Type != TypeSystem && !codeRe.MatchString(f.Code) {
				errs[fp+".code"] = fmt.Sprintf("«%s»: код должен начинаться с латинской буквы и содержать только a-z, 0-9, _", name)
			}
			if f.Type != TypeSystem {
				if _, isSys := SystemFieldByKey(f.Code); isSys {
					errs[fp+".code"] = fmt.Sprintf("«%s»: код «%s» зарезервирован системным полем", name, f.Code)
				}
			}
			if codes[f.Code] {
				errs[fp+".code"] = fmt.Sprintf("«%s»: код «%s» уже используется на форме", name, f.Code)
			}
			codes[f.Code] = true

			switch f.Type {
			case TypeSelect, TypeRadio, TypeMultiselect:
				if len(f.Options) == 0 {
					errs[fp+".options"] = fmt.Sprintf("«%s»: нужен хотя бы один вариант", name)
				}
				seen := map[string]bool{}
				for _, o := range f.Options {
					if o.Value == "" || seen[o.Value] {
						errs[fp+".options"] = fmt.Sprintf("«%s»: значения вариантов должны быть непустыми и уникальными", name)
					}
					seen[o.Value] = true
				}
			case TypeTable:
				if len(f.Columns) == 0 {
					errs[fp+".columns"] = fmt.Sprintf("«%s»: нужна хотя бы одна колонка", name)
				}
				seen := map[string]bool{}
				for _, c := range f.Columns {
					if !codeRe.MatchString(c.Code) || seen[c.Code] || c.Label == "" {
						errs[fp+".columns"] = fmt.Sprintf("«%s»: у колонок должны быть уникальные коды (a-z, 0-9, _) и названия", name)
					}
					if c.Type != "text" && c.Type != "number" && c.Type != "date" {
						errs[fp+".columns"] = fmt.Sprintf("«%s»: неизвестный тип колонки %s", name, c.Type)
					}
					seen[c.Code] = true
				}
			case TypeNumber, TypeMoney:
				if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
					errs[fp+".min"] = fmt.Sprintf("«%s»: минимум больше максимума", name)
				}
			}
		}
	}
	for bi, b := range s.Blocks {
		for fi, f := range b.Fields {
			name := f.Label
			if name == "" {
				name = f.Code
			}
			if msg := s.validateCondition(f, name); msg != "" {
				errs[fmt.Sprintf("blocks[%d].fields[%d].visible_if", bi, fi)] = msg
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func isBlockKind(k string) bool {
	for _, b := range BlockKinds {
		if b.Kind == k {
			return true
		}
	}
	return false
}

func isFieldType(t string) bool {
	for _, ft := range FieldTypes {
		if ft.Type == t {
			return true
		}
	}
	return false
}
