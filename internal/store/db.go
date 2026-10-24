package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type DB struct { *sql.DB }

func Open(path string) (*DB,error) {
	if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return nil,fmt.Errorf("create database directory: %w",err)}
	db,err:=sql.Open("sqlite",path);if err!=nil{return nil,fmt.Errorf("open sqlite: %w",err)}
	db.SetMaxOpenConns(1);db.SetMaxIdleConns(1)
	for _,q:=range []string{"PRAGMA foreign_keys=ON","PRAGMA journal_mode=WAL","PRAGMA busy_timeout=5000"}{if _,err:=db.Exec(q);err!=nil{_ = db.Close();return nil,fmt.Errorf("configure sqlite: %w",err)}}
	if err:=migrate(db);err!=nil{_ = db.Close();return nil,err};return &DB{DB:db},nil
}
func migrate(db *sql.DB) error {
	if _,err:=db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')))`);err!=nil{return err}
	entries,err:=migrations.ReadDir("migrations");if err!=nil{return err};var names []string
	for _,entry:=range entries{if !entry.IsDir()&&strings.HasSuffix(entry.Name(),".sql"){names=append(names,entry.Name())}};sort.Strings(names)
	for _,name:=range names{
		var count int;if err:=db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=?`,name).Scan(&count);err!=nil{return err};if count>0{continue}
		script,err:=migrations.ReadFile("migrations/"+name);if err!=nil{return err}
		tx,err:=db.Begin();if err!=nil{return err}
		if _,err=tx.Exec(string(script));err==nil{_,err=tx.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`,name)}
		if err!=nil{_ = tx.Rollback();return fmt.Errorf("migration %s: %w",name,err)};if err=tx.Commit();err!=nil{return err}
	};return nil
}
