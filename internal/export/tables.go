package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// TableExportResult is the folder and files produced by ExportTable.
type TableExportResult struct {
	Dir   string   `json:"dir"`
	Files []string `json:"files"`
}

type tableExportColumn struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
	RefTable string   `json:"refTable,omitempty"`
}

type tableExportPayload struct {
	Key        string              `json:"key"`
	Name       string              `json:"name"`
	ExportedAt string              `json:"exportedAt"`
	Columns    []tableExportColumn `json:"columns"`
	Rows       []map[string]any    `json:"rows"`
}

// ExportTable writes exports/tables/<key>/<key>.json and <key>.csv. Export is
// gated on TableIssues so an engine never ingests a half-filled table.
func ExportTable(store *project.Store, tableID string) (TableExportResult, error) {
	tables, err := store.ListTables()
	if err != nil {
		return TableExportResult{}, err
	}
	byID := make(map[string]project.DataTable, len(tables))
	var table project.DataTable
	found := false
	for _, item := range tables {
		byID[item.ID] = item
		if item.ID == tableID {
			table = item
			found = true
		}
	}
	if !found {
		return TableExportResult{}, apperr.New("TABLE_NOT_FOUND", "数值表不存在").WithDetail(tableID)
	}

	issues := project.TableIssues(table, byID)
	if len(issues) > 0 {
		lines := make([]string, 0, 5)
		for index, issue := range issues {
			if index >= 5 {
				break
			}
			lines = append(lines, issue.Message)
		}
		return TableExportResult{}, apperr.Newf("TABLE_INVALID", "数值表还有 %d 个问题，先修正再导出", len(issues)).
			WithDetail(strings.Join(lines, "\n"))
	}

	columns := make([]tableExportColumn, 0, len(table.Columns))
	for _, col := range table.Columns {
		item := tableExportColumn{
			Key:      col.Key,
			Name:     col.Name,
			Type:     col.Type,
			Required: col.Required,
			Options:  col.Options,
		}
		if col.Type == project.ColumnRef {
			if target, ok := byID[col.RefTableID]; ok {
				item.RefTable = target.Key
			}
		}
		columns = append(columns, item)
	}

	payload := tableExportPayload{
		Key:        table.Key,
		Name:       table.Name,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Columns:    columns,
		Rows:       exportRows(table),
	}
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return TableExportResult{}, apperr.Wrap("EXPORT_ENCODE", "无法生成数值表 JSON", err)
	}
	jsonBytes = append(jsonBytes, '\n')

	csvBytes, err := exportRowsCSV(table)
	if err != nil {
		return TableExportResult{}, err
	}

	dir := filepath.Join(project.ExportsDir(store.Root()), "tables", table.Key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return TableExportResult{}, apperr.Wrap("IO", "无法创建数值表导出目录", err)
	}
	jsonName := table.Key + ".json"
	csvName := table.Key + ".csv"
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, jsonName), jsonBytes, 0o644); err != nil {
		return TableExportResult{}, apperr.Wrap("IO", "无法写入数值表 JSON", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, csvName), csvBytes, 0o644); err != nil {
		return TableExportResult{}, apperr.Wrap("IO", "无法写入数值表 CSV", err)
	}
	return TableExportResult{Dir: dir, Files: []string{jsonName, csvName}}, nil
}

// exportRows renders a rectangular array: every column appears on every row,
// with a zero value when the cell was never filled.
func exportRows(table project.DataTable) []map[string]any {
	rows := make([]map[string]any, 0, len(table.Rows))
	for _, row := range table.Rows {
		record := make(map[string]any, len(table.Columns)+1)
		record["id"] = row.ID
		for _, col := range table.Columns {
			value, ok := row.Values[col.ID]
			if !ok {
				value = zeroCell(col.Type)
			}
			record[col.Key] = value
		}
		rows = append(rows, record)
	}
	return rows
}

func zeroCell(kind string) any {
	switch kind {
	case project.ColumnInt, project.ColumnNumber:
		return 0
	case project.ColumnBool:
		return false
	default:
		return ""
	}
}

func exportRowsCSV(table project.DataTable) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	header := make([]string, 0, len(table.Columns)+1)
	header = append(header, "id")
	for _, col := range table.Columns {
		header = append(header, col.Key)
	}
	if err := writer.Write(header); err != nil {
		return nil, apperr.Wrap("EXPORT_ENCODE", "无法生成数值表 CSV", err)
	}
	for _, row := range table.Rows {
		record := make([]string, 0, len(table.Columns)+1)
		record = append(record, row.ID)
		for _, col := range table.Columns {
			value, ok := row.Values[col.ID]
			if !ok {
				value = zeroCell(col.Type)
			}
			record = append(record, cellText(value))
		}
		if err := writer.Write(record); err != nil {
			return nil, apperr.Wrap("EXPORT_ENCODE", "无法生成数值表 CSV", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, apperr.Wrap("EXPORT_ENCODE", "无法生成数值表 CSV", err)
	}
	return buffer.Bytes(), nil
}

func cellText(value any) string {
	switch typed := value.(type) {
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}