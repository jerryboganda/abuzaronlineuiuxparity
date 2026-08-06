package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

type column struct {
	Schema       string `json:"schema"`
	Table        string `json:"table"`
	Column       string `json:"column"`
	Ordinal      int    `json:"ordinal"`
	DataType     string `json:"dataType"`
	Nullable     string `json:"nullable"`
	MaxLength    int    `json:"maxLength"`
	NumericScale int    `json:"numericScale"`
}

type manifest struct {
	GeneratedAt string   `json:"generatedAt"`
	Source      string   `json:"source"`
	Columns     []column `json:"columns"`
}

func main() {
	source := flag.String("source", os.Getenv("ABUZAR_SOURCE_SQLSERVER_URL"), "SQL Server connection URL")
	out := flag.String("out", filepath.Join("parity", "catalog", "sqlserver-schema.json"), "manifest output path")
	flag.Parse()
	if *source == "" {
		fmt.Fprintln(os.Stderr, "source is required; pass -source or ABUZAR_SOURCE_SQLSERVER_URL")
		os.Exit(2)
	}

	database, err := sql.Open("sqlserver", *source)
	if err != nil {
		fatal(err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		fatal(err)
	}

	rows, err := database.QueryContext(ctx, `
		SELECT columns.TABLE_SCHEMA, columns.TABLE_NAME, columns.COLUMN_NAME, columns.ORDINAL_POSITION,
		       columns.DATA_TYPE, columns.IS_NULLABLE, COALESCE(columns.CHARACTER_MAXIMUM_LENGTH, 0),
		       COALESCE(columns.NUMERIC_SCALE, 0)
		FROM INFORMATION_SCHEMA.COLUMNS AS columns
		INNER JOIN INFORMATION_SCHEMA.TABLES AS tables
		  ON tables.TABLE_SCHEMA = columns.TABLE_SCHEMA
		 AND tables.TABLE_NAME = columns.TABLE_NAME
		 AND tables.TABLE_TYPE = 'BASE TABLE'
		ORDER BY columns.TABLE_SCHEMA, columns.TABLE_NAME, columns.ORDINAL_POSITION
	`)
	if err != nil {
		fatal(err)
	}
	defer rows.Close()

	result := manifest{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Source: "redacted SQL Server connection", Columns: []column{}}
	for rows.Next() {
		var item column
		if err := rows.Scan(&item.Schema, &item.Table, &item.Column, &item.Ordinal, &item.DataType, &item.Nullable, &item.MaxLength, &item.NumericScale); err != nil {
			fatal(err)
		}
		result.Columns = append(result.Columns, item)
	}
	if err := rows.Err(); err != nil {
		fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	file, err := os.Create(*out)
	if err != nil {
		fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fatal(err)
	}
	fmt.Printf("Wrote %d column records to %s\n", len(result.Columns), *out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
