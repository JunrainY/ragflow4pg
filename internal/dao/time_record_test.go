package dao

import "testing"

func TestDeleteOldestSQLForPostgres(t *testing.T) {
	sql := deleteOldestSQL("postgres")
	expected := "DELETE FROM time_records WHERE id IN (SELECT id FROM time_records ORDER BY id ASC LIMIT ?)"
	if sql != expected {
		t.Fatalf("expected %q, got %q", expected, sql)
	}
}

func TestDeleteOldestSQLForMySQL(t *testing.T) {
	sql := deleteOldestSQL("mysql")
	expected := "DELETE FROM time_records ORDER BY id ASC LIMIT ?"
	if sql != expected {
		t.Fatalf("expected %q, got %q", expected, sql)
	}
}
