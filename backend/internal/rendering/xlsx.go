package rendering

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// XLSXRenderer converts CSV-formatted content into an Excel .xlsx file.
type XLSXRenderer struct{}

func (XLSXRenderer) Render(_ context.Context, document Document) (Output, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"

	content := strings.ReplaceAll(document.Body, "\r\n", "\n")
	rows := strings.Split(strings.TrimSpace(content), "\n")

	for rowIdx, row := range rows {
		cells := parseCSVLine(row)
		for colIdx, cell := range cells {
			colName, _ := excelize.ColumnNumberToName(colIdx + 1)
			cellRef := colName + toString(rowIdx+1)

			if rowIdx == 0 {
				// Bold header row.
				style, _ := f.NewStyle(&excelize.Style{
					Font: &excelize.Font{Bold: true},
				})
				_ = f.SetCellValue(sheet, cellRef, cell)
				_ = f.SetCellStyle(sheet, cellRef, cellRef, style)
			} else {
				_ = f.SetCellValue(sheet, cellRef, cell)
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return Output{}, err
	}

	return Output{
		Bytes:       buf.Bytes(),
		MIMEType:    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Previewable: false,
	}, nil
}

// parseCSVLine does a simple CSV split, handling quoted fields.
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '"' && !inQuotes:
			inQuotes = true
		case ch == '"' && inQuotes:
			if i+1 < len(line) && line[i+1] == '"' {
				current.WriteByte('"')
				i++
			} else {
				inQuotes = false
			}
		case ch == ',' && !inQuotes:
			fields = append(fields, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	fields = append(fields, strings.TrimSpace(current.String()))
	return fields
}

func toString(n int) string {
	return strconv.Itoa(n)
}
