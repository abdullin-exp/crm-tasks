package form

import (
	"testing"
)

func ptr[T any](v T) *T { return &v }

func testSchema() Schema {
	s := Schema{Blocks: []Block{{ID: "b1", Title: "Основное", Kind: BlockFields, Fields: []Field{
		{ID: "1", Type: TypeSystem, System: SysTitle, Label: "Цель", Required: true},
		{ID: "2", Code: "sum", Type: TypeMoney, Label: "Сумма", Min: ptr(0.0)},
		{ID: "3", Code: "kind", Type: TypeRadio, Label: "Тип", Required: true, Options: []Option{{Value: "b2b", Label: "B2B"}, {Value: "b2c", Label: "B2C"}}},
		{ID: "4", Code: "phone", Type: TypePhone, Label: "Телефон"},
		{ID: "5", Code: "tags", Type: TypeMultiselect, Label: "Теги", Options: []Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}},
		{ID: "6", Code: "items", Type: TypeTable, Label: "Товары", Columns: []Column{{Code: "name", Label: "Имя", Type: "text"}, {Code: "qty", Label: "Кол", Type: "number"}}},
		{ID: "7", Code: "deadline", Type: TypeDate, Label: "Дата", Default: "2026-01-01"},
		{ID: "8", Type: TypeInfo, Text: "подсказка"},
	}}, {ID: "b2", Title: "Комментарии", Kind: BlockComments}}}
	s.Normalize()
	return s
}

func TestSchemaValidateOK(t *testing.T) {
	if err := testSchema().Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchemaValidateErrors(t *testing.T) {
	cases := map[string]func(*Schema){
		"duplicate code": func(s *Schema) { s.Blocks[0].Fields[2].Code = "sum" },
		"bad code":       func(s *Schema) { s.Blocks[0].Fields[1].Code = "Сумма" },
		"reserved code":  func(s *Schema) { s.Blocks[0].Fields[1].Code = "title" },
		"no options":     func(s *Schema) { s.Blocks[0].Fields[2].Options = nil },
		"unknown type":   func(s *Schema) { s.Blocks[0].Fields[1].Type = "magic" },
		"unknown system": func(s *Schema) { s.Blocks[0].Fields[0].System = "magic" },
		"dup system": func(s *Schema) {
			s.Blocks[0].Fields = append(s.Blocks[0].Fields, Field{ID: "x", Type: TypeSystem, System: SysTitle, Label: "x"})
		},
		"fields in comments": func(s *Schema) {
			s.Blocks[1].Fields = []Field{{ID: "y", Code: "y", Type: TypeText, Label: "y"}}
		},
		"min > max":        func(s *Schema) { s.Blocks[0].Fields[1].Max = ptr(-1.0) },
		"table no columns": func(s *Schema) { s.Blocks[0].Fields[5].Columns = nil },
		"dup block id":     func(s *Schema) { s.Blocks[1].ID = "b1" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := testSchema()
			mutate(&s)
			s.Normalize()
			if err := s.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateDataCoercion(t *testing.T) {
	s := testSchema()
	out, errs := ValidateData(s, map[string]any{
		"sum":   "1 000,5",
		"kind":  "b2b",
		"phone": "8 (927) 123-45-67",
		"tags":  []any{"a", "b"},
		"items": []any{map[string]any{"name": "Диск", "qty": "2"}, map[string]any{"name": ""}},
		"extra": "ignored",
	}, nil, nil)
	if errs == nil {
		t.Fatal("expected error for '1 000,5'")
	}
	if _, ok := errs["sum"]; !ok {
		t.Fatalf("expected sum error, got %v", errs)
	}

	out, errs = ValidateData(s, map[string]any{
		"sum":   "1000,5",
		"kind":  "b2b",
		"phone": "8 (927) 123-45-67",
		"tags":  []any{"a", "b"},
		"items": []any{map[string]any{"name": "Диск", "qty": "2"}, map[string]any{"name": ""}},
		"extra": "ignored",
	}, nil, nil)
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if out["sum"] != 1000.5 {
		t.Errorf("sum = %v", out["sum"])
	}
	if out["phone"] != "79271234567" {
		t.Errorf("phone = %v", out["phone"])
	}
	if rows := out["items"].([]any); len(rows) != 1 || rows[0].(map[string]any)["qty"] != 2.0 {
		t.Errorf("items = %v", out["items"])
	}
	if out["deadline"] != "2026-01-01" {
		t.Errorf("default not applied: %v", out["deadline"])
	}
	if _, ok := out["extra"]; ok {
		t.Error("unknown key must be dropped on create")
	}
}

func TestValidateDataErrors(t *testing.T) {
	s := testSchema()
	cases := map[string]map[string]any{
		"required radio": {"kind": ""},
		"bad option":     {"kind": "zzz"},
		"negative money": {"kind": "b2b", "sum": -5},
		"bad phone":      {"kind": "b2b", "phone": "123"},
		"bad multi":      {"kind": "b2b", "tags": []any{"zzz"}},
		"bad table num":  {"kind": "b2b", "items": []any{map[string]any{"qty": "abc"}}},
		"bad date":       {"kind": "b2b", "deadline": "25.11.2026"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, errs := ValidateData(s, in, nil, nil); errs == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestValidateDataKeepsExisting(t *testing.T) {
	s := testSchema()
	existing := map[string]any{"kind": "b2c", "removed_field": "старое значение", "sum": 10.0}
	out, errs := ValidateData(s, map[string]any{"sum": 20}, existing, nil)
	if errs != nil {
		t.Fatalf("unexpected: %v", errs)
	}
	if out["kind"] != "b2c" {
		t.Error("absent field must keep existing value on update")
	}
	if out["removed_field"] != "старое значение" {
		t.Error("data of fields removed from schema must be preserved")
	}
	if out["sum"] != 20.0 {
		t.Errorf("sum = %v", out["sum"])
	}
	// явная очистка
	out, _ = ValidateData(s, map[string]any{"sum": nil}, existing, nil)
	if _, ok := out["sum"]; ok {
		t.Error("null must clear value")
	}
}

func TestVisibility(t *testing.T) {
	s := Schema{Blocks: []Block{{ID: "b", Title: "b", Kind: BlockFields, Fields: []Field{
		{ID: "1", Code: "kind", Type: TypeRadio, Label: "Тип", Options: []Option{{Value: "b2b", Label: "B2B"}, {Value: "b2c", Label: "B2C"}}},
		{ID: "2", Code: "inn", Type: TypeText, Label: "ИНН", Required: true, VisibleIf: &Condition{Field: "kind", Op: OpEq, Values: []string{"b2b"}}},
		{ID: "3", Code: "kpp", Type: TypeText, Label: "КПП", Required: true, VisibleIf: &Condition{Field: "inn", Op: OpFilled}},
		{ID: "4", Code: "reason", Type: TypeTextarea, Label: "Причина", Required: true, VisibleIf: &Condition{Field: SysResult, Op: OpIn, Values: []string{"5", "7"}}},
	}}}}
	s.Normalize()
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	// b2c: ИНН скрыт → не обязателен; КПП зависит от ИНН → тоже скрыт
	if _, errs := ValidateData(s, map[string]any{"kind": "b2c"}, nil, nil); errs != nil {
		t.Fatalf("hidden fields must not be required: %v", errs)
	}
	// b2b: ИНН обязателен
	if _, errs := ValidateData(s, map[string]any{"kind": "b2b"}, nil, nil); errs["inn"] == "" {
		t.Fatal("visible required field must fail")
	}
	// ИНН заполнен → КПП появляется и обязателен
	if _, errs := ValidateData(s, map[string]any{"kind": "b2b", "inn": "123"}, nil, nil); errs["kpp"] == "" {
		t.Fatal("chained condition must apply")
	}
	// условие по системному результату
	if _, errs := ValidateData(s, map[string]any{"kind": "b2c"}, nil, map[string]any{SysResult: 7}); errs["reason"] == "" {
		t.Fatal("condition on result must apply")
	}
	if _, errs := ValidateData(s, map[string]any{"kind": "b2c"}, nil, map[string]any{SysResult: 1}); errs != nil {
		t.Fatalf("unexpected: %v", errs)
	}
	// ошибки схемы: ссылка на себя / несуществующее поле / нет значения
	for _, c := range []*Condition{{Field: "inn", Op: OpEq, Values: []string{"x"}}, {Field: "nope", Op: OpFilled}, {Field: "kind", Op: OpEq}} {
		bad := s
		bad.Blocks = []Block{{ID: "b", Title: "b", Kind: BlockFields, Fields: append([]Field{}, s.Blocks[0].Fields...)}}
		bad.Blocks[0].Fields[1].VisibleIf = c
		if bad.Validate() == nil {
			t.Fatalf("expected schema error for %+v", c)
		}
	}
}
