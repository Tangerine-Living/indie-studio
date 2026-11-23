package project

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"pixelbench/internal/apperr"
)

// Column types for game-data tables. Values live per row in Values, keyed by
// column ID so renaming a column key never detaches its data.
const (
	ColumnText   = "text"
	ColumnInt    = "int"
	ColumnNumber = "number"
	ColumnBool   = "bool"
	ColumnEnum   = "enum"
	ColumnRef    = "ref"
)

var tableColumnTypes = []string{ColumnText, ColumnInt, ColumnNumber, ColumnBool, ColumnEnum, ColumnRef}

// TableColumn describes one field of every row in a table.
type TableColumn struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Required   bool     `json:"required"`
	Options    []string `json:"options"`
	RefTableID string   `json:"refTableId,omitempty"`
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
	Note       string   `json:"note,omitempty"`
}

// TableRow is one game-data record. Values use column IDs so column renames
// and reordering never move a value onto the wrong field.
type TableRow struct {
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

// DataTable is a typed game-data table (items, skills, enemies…). One JSON
// file per table keeps diffs small and merge-friendly.
type DataTable struct {
	SchemaVersion int           `json:"schemaVersion"`
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Key           string        `json:"key"`
	Note          string        `json:"note"`
	Columns       []TableColumn `json:"columns"`
	Rows          []TableRow    `json:"rows"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// TableIssue is one blocking problem that keeps a table from exporting.
type TableIssue struct {
	RowID    string `json:"rowId,omitempty"`
	ColumnID string `json:"columnId,omitempty"`
	Message  string `json:"message"`
}

var tableKeyPattern = regexp.MustCompile(`[^a-z0-9_]+`)

// NormalizeTableKey turns any label into a safe engine-facing field name.
// Exported as a helper because the frontend mirrors the same rule.
func NormalizeTableKey(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	key = tableKeyPattern.ReplaceAllString(key, "_")
	key = strings.Trim(key, "_")
	if key == "" {
		key = "table"
	}
	if key[0] >= '0' && key[0] <= '9' {
		key = "t_" + key
	}
	if key == "id" {
		key = "id_value"
	}
	if len(key) > 64 {
		key = strings.TrimRight(key[:64], "_")
		if key == "" {
			key = "table"
		}
	}
	return key
}

func normalizeColumnType(raw string) string {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if slices.Contains(tableColumnTypes, kind) {
		return kind
	}
	return ColumnText
}

// ValidateTable normalises identifiers, cell types and limits, and validates
// the structure of one table. Data problems (required, ranges, missing refs)
// are reported by TableIssues so editing is never blocked mid-entry.
func ValidateTable(t *DataTable) error {
	if t == nil {
		return apperr.New("INVALID_TABLE", "数值表数据为空")
	}
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return apperr.New("INVALID_TABLE", "数值表名称不能为空")
	}
	if len([]rune(t.Name)) > 80 {
		return apperr.New("INVALID_TABLE", "数值表名称不能超过 80 个字符")
	}
	if t.ID == "" {
		t.ID = NewID()
	}
	if !ValidID(t.ID) {
		return apperr.New("INVALID_TABLE", "数值表 ID 不合法")
	}
	t.Key = NormalizeTableKey(t.Key)
	t.Note = strings.TrimSpace(t.Note)
	if len([]rune(t.Note)) > 2000 {
		t.Note = string([]rune(t.Note)[:2000])
	}

	if len(t.Columns) > 64 {
		return apperr.New("INVALID_TABLE", "一张数值表最多 64 列")
	}
	seenColumnIDs := map[string]bool{}
	seenKeys := map[string]bool{}
	for i := range t.Columns {
		col := &t.Columns[i]
		if col.ID == "" || !ValidID(col.ID) || seenColumnIDs[col.ID] {
			col.ID = NewID()
		}
		seenColumnIDs[col.ID] = true

		col.Name = strings.TrimSpace(col.Name)
		if col.Name == "" {
			col.Name = fmt.Sprintf("第 %d 列", i+1)
		}
		if len([]rune(col.Name)) > 40 {
			col.Name = string([]rune(col.Name)[:40])
		}

		key := NormalizeTableKey(col.Key)
		base := key
		for suffix := 2; seenKeys[key]; suffix++ {
			key = fmt.Sprintf("%s_%d", base, suffix)
		}
		seenKeys[key] = true
		col.Key = key

		col.Type = normalizeColumnType(col.Type)
		col.Note = strings.TrimSpace(col.Note)
		if len([]rune(col.Note)) > 200 {
			col.Note = string([]rune(col.Note)[:200])
		}
		switch col.Type {
		case ColumnEnum:
			col.Options = cleanStrings(col.Options, 64, 40)
			col.RefTableID = ""
			col.Min = nil
			col.Max = nil
		case ColumnRef:
			col.Options = nil
			col.RefTableID = strings.TrimSpace(col.RefTableID)
			if col.RefTableID != "" && !ValidID(col.RefTableID) {
				col.RefTableID = ""
			}
			col.Min = nil
			col.Max = nil
		case ColumnInt, ColumnNumber:
			col.Options = nil
			col.RefTableID = ""
			if col.Min != nil && col.Max != nil && *col.Min > *col.Max {
				col.Min, col.Max = col.Max, col.Min
			}
		default:
			col.Options = nil
			col.RefTableID = ""
			col.Min = nil
			col.Max = nil
		}
		if col.Options == nil {
			col.Options = []string{}
		}
	}

	if len(t.Rows) > 5000 {
		return apperr.New("INVALID_TABLE", "一张数值表最多 5000 行")
	}
	seenRows := map[string]bool{}
	for i := range t.Rows {
		row := &t.Rows[i]
		if row.ID == "" || !ValidID(row.ID) || seenRows[row.ID] {
			row.ID = NewID()
		}
		seenRows[row.ID] = true
		row.Values = normalizeRowValues(row.Values, t.Columns)
	}

	t.SchemaVersion = SchemaVersion
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	return nil
}

// normalizeRowValues keeps only values whose type matches their column. Values
// for columns that no longer exist are dropped, which is what removing a
// column should do without a second migration pass.
func normalizeRowValues(values map[string]any, columns []TableColumn) map[string]any {
	out := make(map[string]any, len(values))
	for _, col := range columns {
		raw, ok := values[col.ID]
		if !ok || raw == nil {
			continue
		}
		switch col.Type {
		case ColumnText:
			if text, ok := raw.(string); ok {
				out[col.ID] = capRunes(text, 2000)
			}
		case ColumnEnum:
			if text, ok := raw.(string); ok {
				out[col.ID] = capRunes(strings.TrimSpace(text), 40)
			}
		case ColumnRef:
			if text, ok := raw.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					out[col.ID] = text
				}
			}
		case ColumnInt, ColumnNumber:
			if number, ok := cellNumber(raw); ok {
				if col.Type == ColumnInt {
					number = math.Trunc(number)
				}
				out[col.ID] = number
			}
		case ColumnBool:
			if flag, ok := raw.(bool); ok {
				out[col.ID] = flag
			}
		}
	}
	return out
}

func capRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func cellNumber(raw any) (float64, bool) {
	var number float64
	switch value := raw.(type) {
	case float64:
		number = value
	case float32:
		number = float64(value)
	case int:
		number = float64(value)
	case int64:
		number = float64(value)
	case json.Number:
		parsed, err := value.Float64()
		if err != nil {
			return 0, false
		}
		number = parsed
	default:
		return 0, false
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func (r TableRow) cell(columnID string) (any, bool) {
	value, ok := r.Values[columnID]
	return value, ok
}

func cellEmpty(value any, present bool) bool {
	if !present || value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

// TableIssues lists every problem that should stop an export. byID maps table
// IDs to tables so reference columns can be checked without disk access.
func TableIssues(t DataTable, byID map[string]DataTable) []TableIssue {
	issues := make([]TableIssue, 0, 16)
	add := func(rowID, columnID, message string) {
		if len(issues) >= 200 {
			return
		}
		issues = append(issues, TableIssue{RowID: rowID, ColumnID: columnID, Message: message})
	}

	for _, col := range t.Columns {
		switch col.Type {
		case ColumnEnum:
			if len(col.Options) == 0 {
				add("", col.ID, fmt.Sprintf("列「%s」是枚举，但还没有可选值", col.Name))
			}
		case ColumnRef:
			if col.RefTableID == "" {
				add("", col.ID, fmt.Sprintf("列「%s」还没有选择要引用的表", col.Name))
				continue
			}
			if _, ok := byID[col.RefTableID]; !ok {
				add("", col.ID, fmt.Sprintf("列「%s」引用的表已被删除", col.Name))
			}
		}
	}

	refRows := map[string]map[string]bool{}
	for _, col := range t.Columns {
		if col.Type != ColumnRef || col.RefTableID == "" {
			continue
		}
		target, ok := byID[col.RefTableID]
		if !ok {
			continue
		}
		if _, cached := refRows[col.RefTableID]; !cached {
			ids := make(map[string]bool, len(target.Rows))
			for _, row := range target.Rows {
				ids[row.ID] = true
			}
			refRows[col.RefTableID] = ids
		}
	}

	for _, row := range t.Rows {
		for _, col := range t.Columns {
			value, present := row.cell(col.ID)
			if col.Required && cellEmpty(value, present) {
				add(row.ID, col.ID, fmt.Sprintf("「%s」是必填项", col.Name))
				continue
			}
			if !present {
				continue
			}
			switch col.Type {
			case ColumnInt, ColumnNumber:
				number, ok := cellNumber(value)
				if !ok {
					add(row.ID, col.ID, fmt.Sprintf("「%s」不是数字", col.Name))
					continue
				}
				if col.Min != nil && number < *col.Min {
					add(row.ID, col.ID, fmt.Sprintf("「%s」不能小于 %s", col.Name, formatNumber(*col.Min)))
				}
				if col.Max != nil && number > *col.Max {
					add(row.ID, col.ID, fmt.Sprintf("「%s」不能大于 %s", col.Name, formatNumber(*col.Max)))
				}
			case ColumnEnum:
				text, ok := value.(string)
				if !ok || !slices.Contains(col.Options, text) {
					add(row.ID, col.ID, fmt.Sprintf("「%s」的值不在可选范围内", col.Name))
				}
			case ColumnRef:
				text, ok := value.(string)
				if !ok || text == "" {
					continue
				}
				if ids, has := refRows[col.RefTableID]; has && !ids[text] {
					add(row.ID, col.ID, fmt.Sprintf("「%s」引用的行已被删除", col.Name))
				}
			}
		}
	}
	return issues
}

func formatNumber(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
}