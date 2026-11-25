package form

import (
	"fmt"
	"strings"
)

// Операторы условия видимости
const (
	OpEq     = "eq"     // равно (для мультивыбора — содержит)
	OpNeq    = "neq"    // не равно
	OpIn     = "in"     // одно из значений
	OpNotIn  = "not_in" // ни одно из значений
	OpFilled = "filled" // заполнено
	OpEmpty  = "empty"  // не заполнено
)

// Condition — условие показа поля: поле видно, только если значение поля Field удовлетворяет условию.
// Field — код пользовательского поля или ключ системного (result_id, priority, executor_user_id, ...).
type Condition struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

var conditionSystemKeys = map[string]bool{
	SysResult: true, SysPriority: true, SysExecutor: true, SysContractor: true, SysContact: true,
	SysSolvePlanAt: true, SysTitle: true, SysDescription: true, SysObservers: true,
}

func validOp(op string) bool {
	switch op {
	case OpEq, OpNeq, OpIn, OpNotIn, OpFilled, OpEmpty:
		return true
	}
	return false
}

// validateCondition проверяет условие поля при сохранении схемы.
func (s Schema) validateCondition(f Field, name string) string {
	c := f.VisibleIf
	if c == nil {
		return ""
	}
	if !validOp(c.Op) {
		return fmt.Sprintf("«%s»: неизвестный оператор условия «%s»", name, c.Op)
	}
	if c.Field == "" {
		return fmt.Sprintf("«%s»: в условии не выбрано поле", name)
	}
	if c.Field == f.Code {
		return fmt.Sprintf("«%s»: условие не может ссылаться на само поле", name)
	}
	if !conditionSystemKeys[c.Field] && !s.hasCustom(c.Field) {
		return fmt.Sprintf("«%s»: поле условия «%s» отсутствует на форме", name, c.Field)
	}
	if c.Op != OpFilled && c.Op != OpEmpty && len(c.Values) == 0 {
		return fmt.Sprintf("«%s»: в условии не указано значение", name)
	}
	return ""
}

func (s Schema) hasCustom(code string) bool {
	for _, f := range s.CustomFields() {
		if f.Code == code {
			return true
		}
	}
	return false
}

// IsVisible вычисляет видимость поля по значениям задачи (data + системные поля).
// Учитывается цепочка: если скрыто поле-источник, скрыто и зависимое.
func (s Schema) IsVisible(f Field, values map[string]any) bool {
	return s.isVisible(f, values, 0)
}

func (s Schema) isVisible(f Field, values map[string]any, depth int) bool {
	c := f.VisibleIf
	if c == nil {
		return true
	}
	if depth > 10 { // защита от циклов
		return false
	}
	if src, ok := s.fieldByCode(c.Field); ok && !s.isVisible(src, values, depth+1) {
		return false
	}
	vals := valueStrings(values[c.Field])
	filled := len(vals) > 0
	match := func() bool {
		for _, v := range vals {
			for _, want := range c.Values {
				if v == want {
					return true
				}
			}
		}
		return false
	}
	switch c.Op {
	case OpFilled:
		return filled
	case OpEmpty:
		return !filled
	case OpEq, OpIn:
		return match()
	case OpNeq, OpNotIn:
		return !match()
	}
	return true
}

func (s Schema) fieldByCode(code string) (Field, bool) {
	for _, b := range s.Blocks {
		for _, f := range b.Fields {
			if f.Code == code && f.Type != TypeInfo {
				return f, true
			}
		}
	}
	return Field{}, false
}

// valueStrings приводит значение к списку строк для сравнения (пустые значения отбрасываются).
func valueStrings(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		var res []string
		for _, x := range t {
			if s := strings.TrimSpace(toString(x)); s != "" {
				res = append(res, s)
			}
		}
		return res
	case []string:
		return t
	case bool:
		return []string{toString(t)} // false — тоже значение
	case *string:
		if t == nil {
			return nil
		}
		return valueStrings(*t)
	case *int:
		if t == nil {
			return nil
		}
		return []string{fmt.Sprint(*t)}
	case int:
		return []string{fmt.Sprint(t)}
	}
	if s := strings.TrimSpace(toString(v)); s != "" {
		return []string{s}
	}
	return nil
}
