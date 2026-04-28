package dao

import (
	"strings"
	"testing"

	"ragflow/internal/server"
)

func TestBuildDSNForPostgres(t *testing.T) {
	cfg := server.DatabaseConfig{
		Driver:   "postgres",
		Host:     "localhost",
		Port:     5432,
		Database: "rag_flow",
		Username: "rag_flow",
		Password: "secret",
	}

	dsn := buildDSN(cfg)

	for _, fragment := range []string{
		"host=localhost",
		"user=rag_flow",
		"password=secret",
		"dbname=rag_flow",
		"port=5432",
		"sslmode=disable",
	} {
		if !strings.Contains(dsn, fragment) {
			t.Fatalf("dsn %q missing fragment %q", dsn, fragment)
		}
	}
}

func TestBuildDSNForMySQL(t *testing.T) {
	cfg := server.DatabaseConfig{
		Driver:   "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "rag_flow",
		Username: "root",
		Password: "secret",
		Charset:  "utf8mb4",
	}

	dsn := buildDSN(cfg)

	if !strings.Contains(dsn, "root:secret@tcp(localhost:3306)/rag_flow") {
		t.Fatalf("unexpected mysql dsn: %q", dsn)
	}
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Fatalf("expected charset in mysql dsn: %q", dsn)
	}
}
