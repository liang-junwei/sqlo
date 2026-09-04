package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Formatter 输出格式化器接口
type Formatter interface {
	Format(w io.Writer, result *QueryResult) error
}

// QueryResult 查询结果
type QueryResult struct {
	Columns []string        `json:"columns,omitempty"`
	Rows    [][]interface{} `json:"rows,omitempty"`
	Message string          `json:"message,omitempty"` // 非查询语句的成功消息
}

// GetFormatter 根据格式名称返回对应的格式化器
func GetFormatter(format string) (Formatter, error) {
	switch strings.ToLower(format) {
	case "table":
		return &TableFormatter{}, nil
	case "json":
		return &JSONFormatter{}, nil
	case "yaml":
		return &YAMLFormatter{}, nil
	case "csv":
		return &CSVFormatter{}, nil
	default:
		return nil, fmt.Errorf("不支持的输出格式: %s (支持: table, json, yaml, csv)", format)
	}
}

// TableFormatter 表格格式化器
type TableFormatter struct{}

func (f *TableFormatter) Format(w io.Writer, result *QueryResult) error {
	if result.Message != "" {
		fmt.Fprintln(w, result.Message)
		return nil
	}

	if len(result.Columns) == 0 {
		return nil
	}

	// 计算每列最大宽度
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}

	for _, row := range result.Rows {
		for i, val := range row {
			s := formatValue(val)
			if len(s) > widths[i] {
				widths[i] = len(s)
			}
		}
	}

	// 打印表头
	printRow(w, result.Columns, widths)
	printSeparator(w, widths)

	// 打印数据行
	for _, row := range result.Rows {
		values := make([]string, len(row))
		for i, val := range row {
			values[i] = formatValue(val)
		}
		printRow(w, values, widths)
	}

	fmt.Fprintf(w, "\n共 %d 行\n", len(result.Rows))
	return nil
}

func printRow(w io.Writer, values []string, widths []int) {
	for i, val := range values {
		if i > 0 {
			fmt.Fprint(w, " | ")
		}
		fmt.Fprintf(w, "%-*s", widths[i], val)
	}
	fmt.Fprintln(w)
}

func printSeparator(w io.Writer, widths []int) {
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, "-+-")
		}
		fmt.Fprint(w, strings.Repeat("-", width))
	}
	fmt.Fprintln(w)
}

func formatValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", val)
}

// JSONFormatter JSON 格式化器
type JSONFormatter struct{}

func (f *JSONFormatter) Format(w io.Writer, result *QueryResult) error {
	// 转换为更易读的格式
	output := make([]map[string]interface{}, 0, len(result.Rows))

	for _, row := range result.Rows {
		record := make(map[string]interface{})
		for i, col := range result.Columns {
			if i < len(row) {
				record[col] = row[i]
			}
		}
		output = append(output, record)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// YAMLFormatter YAML 格式化器
type YAMLFormatter struct{}

func (f *YAMLFormatter) Format(w io.Writer, result *QueryResult) error {
	// 转换为更易读的格式
	output := make([]map[string]interface{}, 0, len(result.Rows))

	for _, row := range result.Rows {
		record := make(map[string]interface{})
		for i, col := range result.Columns {
			if i < len(row) {
				record[col] = row[i]
			}
		}
		output = append(output, record)
	}

	encoder := yaml.NewEncoder(w)
	defer encoder.Close()
	return encoder.Encode(output)
}

// CSVFormatter CSV 格式化器
type CSVFormatter struct{}

func (f *CSVFormatter) Format(w io.Writer, result *QueryResult) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// 写入表头
	if err := writer.Write(result.Columns); err != nil {
		return err
	}

	// 写入数据行
	for _, row := range result.Rows {
		record := make([]string, len(row))
		for i, val := range row {
			record[i] = formatValue(val)
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}
