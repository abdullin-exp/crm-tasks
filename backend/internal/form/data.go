package form

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var nonDigits = regexp.MustCompile(`[^\d]`)

// ValidateData проверяет и приводит значения пользовательских полей к типам схемы.
// existing — текущие данные задачи (nil при создании): ключи, которых больше нет в схеме,
// сохраняются, чтобы изменение формы не теряло данные уже созданных задач.
// Отсутствующее во входных данных поле при создании получает значение по умолчанию,
// при обновлении — сохраняет текущее значение.
func ValidateData(s Schema, input, existing, sys map[string]any) (map[string]any, Errors) {
	out := map[string]any{}
	for k, v := range existing {
		out[k] = v
	}
	errs := Errors{}

	// 1. приведение типов
	coerced := map[string]any{}
	for _, f := range s.CustomFields() {
		raw, present := input[f.Code]
		if !present {
			if existing != nil {
				raw = existing[f.Code]
			} else {
				raw = f.Default
			}
		}
		val, err := coerce(f, raw)
		if err != nil {
			errs[f.Code] = fmt.Sprintf("«%s»: %s", f.Label, err.Error())
			continue
		}
		coerced[f.Code] = val
	}

	// 2. обязательность — только для видимых полей (условия считаются по приведённым значениям)
	values := map[string]any{}
	for k, v := range sys {
		values[k] = v
	}
	for k, v := range coerced {
		values[k] = v
	}
	for _, f := range s.CustomFields() {
		val, ok := coerced[f.Code]
		if !ok {
			continue
		}
		if f.Required && isEmpty(f, val) && s.IsVisible(f, values) {
			errs[f.Code] = fmt.Sprintf("«%s»: обязательное поле", f.Label)
			continue
		}
		if val == nil {
			delete(out, f.Code)
		} else {
			out[f.Code] = val
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return out, nil
}

func isEmpty(f Field, v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case []string:
		return len(t) == 0
	case []map[string]any:
		return len(t) == 0
	case bool:
		return f.Type == TypeCheckbox && !t // обязательный флажок должен быть отмечен
	}
	return false
}

func coerce(f Field, raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	if s, ok := raw.(string); ok && strings.TrimSpace(s) == "" && f.Type != TypeCheckbox {
		return nil, nil
	}

	switch f.Type {
	case TypeText, TypeTextarea:
		return toString(raw), nil

	case TypePhone:
		s := toString(raw)
		digits := nonDigits.ReplaceAllString(s, "")
		if len(digits) < 10 || len(digits) > 15 {
			return nil, fmt.Errorf("некорректный телефон")
		}
		if len(digits) == 11 && digits[0] == '8' {
			digits = "7" + digits[1:]
		}
		return digits, nil

	case TypeEmail:
		s := strings.TrimSpace(toString(raw))
		a, err := mail.ParseAddress(s)
		if err != nil || a.Address != s {
			return nil, fmt.Errorf("некорректный e-mail")
		}
		return s, nil

	case TypeURL:
		s := strings.TrimSpace(toString(raw))
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("ссылка должна начинаться с http:// или https://")
		}
		return s, nil

	case TypeNumber, TypeMoney:
		n, err := toFloat(raw)
		if err != nil {
			return nil, fmt.Errorf("ожидается число")
		}
		if f.Min != nil && n < *f.Min {
			return nil, fmt.Errorf("значение меньше %v", *f.Min)
		}
		if f.Max != nil && n > *f.Max {
			return nil, fmt.Errorf("значение больше %v", *f.Max)
		}
		return n, nil

	case TypeDate:
		s := toString(raw)
		if len(s) > 10 {
			s = s[:10]
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, fmt.Errorf("ожидается дата в формате ГГГГ-ММ-ДД")
		}
		return s, nil

	case TypeDatetime:
		t, err := time.Parse(time.RFC3339, toString(raw))
		if err != nil {
			return nil, fmt.Errorf("ожидается дата и время в формате RFC3339")
		}
		return t.Format(time.RFC3339), nil

	case TypeCheckbox:
		switch t := raw.(type) {
		case bool:
			return t, nil
		case string:
			b, err := strconv.ParseBool(t)
			if err != nil {
				return nil, fmt.Errorf("ожидается true/false")
			}
			return b, nil
		case float64:
			return t != 0, nil
		}
		return nil, fmt.Errorf("ожидается true/false")

	case TypeSelect, TypeRadio:
		s := toString(raw)
		if !hasOption(f, s) {
			return nil, fmt.Errorf("недопустимое значение «%s»", s)
		}
		return s, nil

	case TypeMultiselect:
		arr, ok := raw.([]any)
		if !ok {
			if ss, ok2 := raw.([]string); ok2 {
				for _, s := range ss {
					arr = append(arr, s)
				}
			} else {
				return nil, fmt.Errorf("ожидается список значений")
			}
		}
		res := make([]any, 0, len(arr))
		for _, v := range arr {
			s := toString(v)
			if !hasOption(f, s) {
				return nil, fmt.Errorf("недопустимое значение «%s»", s)
			}
			res = append(res, s)
		}
		return res, nil

	case TypeUser, TypeContractor:
		s := toString(raw)
		if _, err := uuid.Parse(s); err != nil {
			return nil, fmt.Errorf("ожидается идентификатор (uuid)")
		}
		return s, nil

	case TypeTable:
		rows, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("ожидается список строк")
		}
		res := make([]any, 0, len(rows))
		for i, r := range rows {
			m, ok := r.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("строка %d: ожидается объект", i+1)
			}
			row := map[string]any{}
			for _, c := range f.Columns {
				v, present := m[c.Code]
				if !present || v == nil || toString(v) == "" {
					continue
				}
				switch c.Type {
				case "number":
					n, err := toFloat(v)
					if err != nil {
						return nil, fmt.Errorf("строка %d, «%s»: ожидается число", i+1, c.Label)
					}
					row[c.Code] = n
				case "date":
					s := toString(v)
					if len(s) > 10 {
						s = s[:10]
					}
					if _, err := time.Parse("2006-01-02", s); err != nil {
						return nil, fmt.Errorf("строка %d, «%s»: ожидается дата", i+1, c.Label)
					}
					row[c.Code] = s
				default:
					row[c.Code] = toString(v)
				}
			}
			if len(row) > 0 {
				res = append(res, row)
			}
		}
		return res, nil
	}
	return nil, fmt.Errorf("неподдерживаемый тип поля %s", f.Type)
}

func hasOption(f Field, v string) bool {
	for _, o := range f.Options {
		if o.Value == v {
			return true
		}
	}
	return false
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

func toFloat(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case json.Number:
		return t.Float64()
	case string:
		return strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(t), ",", "."), 64)
	}
	return 0, fmt.Errorf("not a number")
}
