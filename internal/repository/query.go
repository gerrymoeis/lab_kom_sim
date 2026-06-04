package repository

import (
	"fmt"
	"reflect"
	"strings"
)

func fieldPtrs(v any) []any {
	rv := reflect.ValueOf(v).Elem()
	ptrs := make([]any, rv.NumField())
	for i := range ptrs {
		ptrs[i] = rv.Field(i).Addr().Interface()
	}
	return ptrs
}

func formatColumns(cols []string) string {
	return strings.Join(cols, ", ")
}

func getOne[T any](db DBTX, table string, columns []string, where string, args ...any) (*T, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s", formatColumns(columns), table, where)
	var result T
	if err := db.QueryRow(query, args...).Scan(fieldPtrs(&result)...); err != nil {
		return nil, err
	}
	return &result, nil
}

func getByField[T any](db DBTX, table string, columns []string, field string, value any) (*T, error) {
	return getOne[T](db, table, columns, fmt.Sprintf("%s = ?", field), value)
}
