package database

import (
	"fmt"
	"strings"
)

// SelectBuilder 链式查询构建器
type SelectBuilder struct {
	table        string
	columns      []string
	whereClauses []string
	whereArgs    []interface{}
	orderBy      []string
	limitVal     *int
	offsetVal    *int
}

// Select 开始构建 SELECT 查询
func Select(table string, columns ...string) *SelectBuilder {
	if len(columns) == 0 {
		columns = []string{"*"}
	}
	return &SelectBuilder{
		table:   table,
		columns: columns,
	}
}

// Where 添加 WHERE 条件
func (b *SelectBuilder) Where(condition string, args ...interface{}) *SelectBuilder {
	b.whereClauses = append(b.whereClauses, condition)
	b.whereArgs = append(b.whereArgs, args...)
	return b
}

// AndWhere 链式添加 AND 条件（语法糖）
func (b *SelectBuilder) AndWhere(condition string, args ...interface{}) *SelectBuilder {
	return b.Where(condition, args...)
}

// OrderBy 添加排序
func (b *SelectBuilder) OrderBy(expr string) *SelectBuilder {
	b.orderBy = append(b.orderBy, expr)
	return b
}

// Limit 设置限制条数
func (b *SelectBuilder) Limit(n int) *SelectBuilder {
	b.limitVal = &n
	return b
}

// Offset 设置偏移量
func (b *SelectBuilder) Offset(n int) *SelectBuilder {
	b.offsetVal = &n
	return b
}

// Build 构建最终 SQL 和参数
func (b *SelectBuilder) Build() (sql string, args []interface{}) {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(b.columns, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(b.table)

	if len(b.whereClauses) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(b.whereClauses, " AND "))
	}

	if len(b.orderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(b.orderBy, ", "))
	}

	if b.limitVal != nil {
		sb.WriteString(" LIMIT ")
		sb.WriteString(fmt.Sprintf("%d", *b.limitVal))
	}

	if b.offsetVal != nil {
		sb.WriteString(" OFFSET ")
		sb.WriteString(fmt.Sprintf("%d", *b.offsetVal))
	}

	return sb.String(), b.whereArgs
}

// InsertBuilder (增强：支持单行/批量插入)

// InsertBuilder 插入构建器
type InsertBuilder struct {
	table   string
	columns []string
	values  [][]interface{}
}

// Insert 开始构建 INSERT
func Insert(table string) *InsertBuilder {
	return &InsertBuilder{table: table}
}

// Set 设置列名和第一行数据（链式）
func (b *InsertBuilder) Set(columns []string, values ...interface{}) *InsertBuilder {
	b.columns = columns
	b.values = append(b.values, values)
	return b
}

// AddRow 追加单行数据（用于批量插入）
func (b *InsertBuilder) AddRow(values ...interface{}) *InsertBuilder {
	b.values = append(b.values, values)
	return b
}

// AddRows 追加多行数据（用于批量插入）
func (b *InsertBuilder) AddRows(rows ...[]interface{}) *InsertBuilder {
	b.values = append(b.values, rows...)
	return b
}

// Build 构建 INSERT SQL（支持单行与多行批量插入，自动递增占位符）
func (b *InsertBuilder) Build() (sql string, args []interface{}, err error) {
	if len(b.columns) == 0 || len(b.values) == 0 {
		return "", nil, fmt.Errorf("columns and values required")
	}

	colCount := len(b.columns)
	// 校验每行数据长度是否与列数一致
	for i, row := range b.values {
		if len(row) != colCount {
			return "", nil, fmt.Errorf("row %d value count %d mismatch columns count %d", i, len(row), colCount)
		}
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(b.table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(b.columns, ", "))
	sb.WriteString(") VALUES ")

	// 预分配参数切片容量，避免频繁扩容
	args = make([]interface{}, 0, colCount*len(b.values))
	paramIdx := 1

	for i, row := range b.values {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for j := range row {
			if j > 0 {
				sb.WriteString(", ")
			}
			// PostgreSQL 风格占位符: $1, $2, $3...
			sb.WriteString(fmt.Sprintf("$%d", paramIdx))
			args = append(args, row[j])
			paramIdx++
		}
		sb.WriteString(")")
	}

	return sb.String(), args, nil
}

// UpdateBuilder 更新构建器
type UpdateBuilder struct {
	table        string
	sets         []string
	setArgs      []interface{}
	whereClauses []string
	whereArgs    []interface{}
}

// Update 开始构建 UPDATE
func Update(table string) *UpdateBuilder {
	return &UpdateBuilder{table: table}
}

// Set 添加更新字段
func (b *UpdateBuilder) Set(column string, value interface{}) *UpdateBuilder {
	idx := len(b.setArgs) + 1
	b.sets = append(b.sets, fmt.Sprintf("%s = $%d", column, idx))
	b.setArgs = append(b.setArgs, value)
	return b
}

// Where 添加 WHERE 条件
func (b *UpdateBuilder) Where(condition string, args ...interface{}) *UpdateBuilder {
	b.whereClauses = append(b.whereClauses, condition)
	b.whereArgs = append(b.whereArgs, args...)
	return b
}

// Build 构建最终 SQL
func (b *UpdateBuilder) Build() (sql string, args []interface{}) {
	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(b.table)
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(b.sets, ", "))

	if len(b.whereClauses) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(b.whereClauses, " AND "))
		// 合并参数：setArgs + whereArgs
		args = append(b.setArgs, b.whereArgs...)
		return sb.String(), args
	}

	return sb.String(), b.setArgs
}

// DeleteBuilder 删除构建器
type DeleteBuilder struct {
	table        string
	whereClauses []string
	whereArgs    []interface{}
}

// Delete 开始构建 DELETE
func Delete(table string) *DeleteBuilder {
	return &DeleteBuilder{table: table}
}

// Where 添加条件
func (b *DeleteBuilder) Where(condition string, args ...interface{}) *DeleteBuilder {
	b.whereClauses = append(b.whereClauses, condition)
	b.whereArgs = append(b.whereArgs, args...)
	return b
}

// Build 构建最终 SQL
func (b *DeleteBuilder) Build() (sql string, args []interface{}) {
	var sb strings.Builder
	sb.WriteString("DELETE FROM ")
	sb.WriteString(b.table)

	if len(b.whereClauses) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(b.whereClauses, " AND "))
		return sb.String(), b.whereArgs
	}

	return sb.String(), nil
}
