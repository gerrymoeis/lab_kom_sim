package repository

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"inventaris-lab-kom/internal/models"
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

func getAll[T any](db DBTX, table string, columns []string, where string, args ...any) ([]T, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s", formatColumns(columns), table, where)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var result T
		if err := rows.Scan(fieldPtrs(&result)...); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// --- Device shared query (17 columns, JOIN device_types + categories) ---

var deviceFullCols = []string{
	"d.id", "d.device_type_id", "d.asset_code", "COALESCE(d.serial_number,'')",
	"d.condition", "COALESCE(d.location,'')", "d.purchase_date", "COALESCE(d.notes,'')",
	"d.created_at", "d.updated_at",
	"c.name", "c.default_prefix", "dt.name", "dt.asset_code_prefix",
	"COALESCE(d.usage_type, dt.usage_type) AS usage_type",
	"COALESCE(d.usage_type, '') AS usage_type_override",
	"COALESCE(dt.photo,'')",
}
var deviceFullFrom = "FROM devices d JOIN device_types dt ON d.device_type_id = dt.id JOIN categories c ON c.id = dt.category_id"

func scanDeviceRow(db DBTX, where string, args ...any) (*models.Device, error) {
	query := fmt.Sprintf("SELECT %s %s WHERE %s", strings.Join(deviceFullCols, ", "), deviceFullFrom, where)
	var d models.Device
	var pDate sql.NullString
	err := db.QueryRow(query, args...).Scan(
		&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.SerialNumber,
		&d.Condition, &d.Location, &pDate, &d.Notes,
		&d.CreatedAt, &d.UpdatedAt,
		&d.CategoryName, &d.CategoryPrefix, &d.DeviceTypeName, &d.DeviceTypePrefix,
		&d.UsageType, &d.UsageTypeOverride, &d.DeviceTypePhoto)
	if err != nil {
		return nil, err
	}
	d.PurchaseDate = parseDate(pDate)
	return &d, nil
}

// --- DeviceType shared queries (JOIN categories) ---

var deviceTypeFullCols = []string{
	"dt.id", "dt.category_id", "c.name", "c.default_prefix", "dt.name", "dt.brand", "dt.model",
	"dt.asset_code_prefix", "dt.usage_type", "dt.default_location", "COALESCE(dt.photo,'')",
	"dt.created_at", "dt.updated_at",
}
var deviceTypeNoPrefixCols = []string{
	"dt.id", "dt.category_id", "c.name", "dt.name", "dt.brand", "dt.model",
	"dt.asset_code_prefix", "dt.usage_type", "dt.default_location", "COALESCE(dt.photo,'')",
	"dt.created_at", "dt.updated_at",
}
var deviceTypeFrom = "FROM device_types dt JOIN categories c ON c.id = dt.category_id"

func scanDeviceTypeRowWithPrefix(db DBTX, where string, args ...any) (*models.DeviceType, error) {
	query := fmt.Sprintf("SELECT %s %s WHERE %s", strings.Join(deviceTypeFullCols, ", "), deviceTypeFrom, where)
	var dt models.DeviceType
	var brand, model, loc, photo sql.NullString
	err := db.QueryRow(query, args...).Scan(
		&dt.ID, &dt.CategoryID, &dt.CategoryName, &dt.CategoryPrefix, &dt.Name, &brand, &model,
		&dt.AssetCodePrefix, &dt.UsageType, &loc, &photo, &dt.CreatedAt, &dt.UpdatedAt)
	if err != nil {
		return nil, err
	}
	dt.Brand = valStr(brand)
	dt.Model = valStr(model)
	dt.DefaultLocation = valStr(loc)
	dt.Photo = valStr(photo)
	return &dt, nil
}

func scanDeviceTypeRowNoPrefix(db DBTX, where string, args ...any) (*models.DeviceType, error) {
	query := fmt.Sprintf("SELECT %s %s WHERE %s", strings.Join(deviceTypeNoPrefixCols, ", "), deviceTypeFrom, where)
	var dt models.DeviceType
	var brand, model, loc, photo sql.NullString
	err := db.QueryRow(query, args...).Scan(
		&dt.ID, &dt.CategoryID, &dt.CategoryName, &dt.Name, &brand, &model,
		&dt.AssetCodePrefix, &dt.UsageType, &loc, &photo, &dt.CreatedAt, &dt.UpdatedAt)
	if err != nil {
		return nil, err
	}
	dt.Brand = valStr(brand)
	dt.Model = valStr(model)
	dt.DefaultLocation = valStr(loc)
	dt.Photo = valStr(photo)
	return &dt, nil
}

func parseDate(s sql.NullString) *time.Time {
	if s.Valid && s.String != "" {
		t, err := time.Parse("2006-01-02", s.String)
		if err == nil {
			return &t
		}
		t, err = time.Parse(time.RFC3339, s.String)
		if err == nil {
			return &t
		}
	}
	return nil
}
