package dao

import (
	"strings"
	"testing"
)

func TestSkillSearchTableSQLForPostgresUsesPostgresDialect(t *testing.T) {
	sql := skillSearchTableSQL("postgres")
	upper := strings.ToUpper(sql)

	for _, forbidden := range []string{"UNIQUE INDEX", " DATETIME", "\n\t\t\tINDEX "} {
		if strings.Contains(upper, forbidden) {
			t.Fatalf("postgres skill_search_configs SQL contains MySQL syntax %q: %s", forbidden, sql)
		}
	}
	if !strings.Contains(upper, "JSONB") {
		t.Fatalf("postgres skill_search_configs SQL should use JSONB: %s", sql)
	}
}

func TestSkillSpaceIndexSQLForPostgresUsesPostgresDialect(t *testing.T) {
	dropSQL, createSQL := skillSpaceIndexSQL("postgres")

	if strings.Contains(strings.ToUpper(dropSQL), " ON ") {
		t.Fatalf("postgres drop index SQL should not use MySQL ON clause: %s", dropSQL)
	}
	if !strings.Contains(strings.ToUpper(createSQL), "CREATE UNIQUE INDEX") {
		t.Fatalf("postgres create index SQL should create a unique index: %s", createSQL)
	}
}
