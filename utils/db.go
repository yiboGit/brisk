package utils

import (
	"database/sql"
	"log"
	"reflect"

	_ "github.com/go-sql-driver/mysql"
)

type AppIdToken struct {
	Appid        string
	RefreshToken string
}

type DB struct {
	DB *sql.DB
}

func GetDB(isProd bool) *DB {
	str := "root:epeijing@tcp(t.epeijing.cn:3306)/db_new?charset=utf8mb4&loc=Asia%2FShanghai"
	if isProd {
		str = "epeijing:B0vZxXWV@tcp(rm-uf6th65g631q9kc94.mysql.rds.aliyuncs.com:3306)/db_new?charset=utf8mb4&loc=Asia%2FShanghai"
	}
	log.Printf("connecting to mysql: %s\n", str)
	cdb, error := sql.Open("mysql", str)
	if error != nil {
		panic(error)
	}
	return &DB{cdb}
}
func (db *DB) Close() {
	db.DB.Close()
}

// SelectColumn 返回struct
func (db *DB) SelectColumn(query string, args ...interface{}) ([]int, error) {
	rows, error := db.DB.Query(query, args...)
	if error != nil {
		log.Printf("query error: %s", query)
		return nil, error
	}
	var result []int
	defer rows.Close()
	for rows.Next() {
		var t int
		rows.Scan(&t)
		result = append(result, t)
	}
	return result, nil
}

// Query 返回 map
func (db *DB) Select(result interface{}, query string, args ...interface{}) error {
	rows, error := db.DB.Query(query, args...)
	if error != nil {
		log.Printf("query error: %s", query)
		return error
	}
	columns, _ := rows.Columns()
	columnsPtrs := make([]interface{}, len(columns))
	for i := 0; i < len(columns); i++ {
		v := reflect.ValueOf(result).Elem()
		fn := v.FieldByName(columns[i])
		columnsPtrs[i] = fn.Addr().Interface()
	}
	defer rows.Close()
	for rows.Next() {
		rows.Scan(columnsPtrs...)
	}
	return nil
}
func (db *DB) QuerySingle(result interface{}, query string, args ...interface{}) error {
	rows, error := db.DB.Query(query, args...)
	if error != nil {
		log.Printf("query error: %s: %v", query, args)
		log.Printf("error: %v", error)
		return error
	}
	defer rows.Close()
	for rows.Next() {
		rows.Scan(result)
	}
	return nil
}

// QueryAll query result as raw string->btyes map
func (db *DB) QueryAll(query string, args ...interface{}) ([]map[string]sql.RawBytes, error) {
	rows, error := db.DB.Query(query, args...)
	if error != nil {
		log.Printf("query all error: %s: %v", query, args)
		log.Printf("error: %v\n", error)
		return nil, error
	}
	columns, error := rows.Columns()
	if error != nil {
		log.Printf("query result has not columns, %v", error)
		return nil, error
	}
	values := make([]sql.RawBytes, len(columns))
	columnsPtrs := make([]interface{}, len(columns))
	for i := range columns {
		columnsPtrs[i] = &values[i]
	}
	var result []map[string]sql.RawBytes
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(columnsPtrs...)
		if err != nil {
			return nil, error
		}
		row := map[string]sql.RawBytes{}
		for i := range columns {
			row[columns[i]] = values[i]
		}
		result = append(result, row)
	}
	return result, nil
}

// Exec  exec sql
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(query, args...)
}
