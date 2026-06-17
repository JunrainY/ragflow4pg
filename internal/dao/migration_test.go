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
	if !strings.Contains(upper, "CONSTRAINT IDX_TENANT_SPACE_EMBD UNIQUE") {
		t.Fatalf("postgres skill_search_configs SQL should enforce tenant/space/embedding uniqueness: %s", sql)
	}
}

func TestSkillSearchTableSQLForMySQLKeepsUpstreamDialect(t *testing.T) {
	sql := skillSearchTableSQL("mysql")
	upper := strings.ToUpper(sql)

	for _, required := range []string{
		"VECTOR_SIMILARITY_WEIGHT FLOAT",
		"FIELD_CONFIG JSON",
		"CREATE_DATE DATETIME",
		"INDEX IDX_TENANT_ID (TENANT_ID)",
		"INDEX IDX_SPACE_ID (SPACE_ID)",
		"UNIQUE INDEX IDX_TENANT_SPACE_EMBD",
	} {
		if !strings.Contains(upper, required) {
			t.Fatalf("mysql skill_search_configs SQL missing %q: %s", required, sql)
		}
	}
	if strings.Contains(upper, "JSONB") || strings.Contains(upper, "DOUBLE PRECISION") {
		t.Fatalf("mysql skill_search_configs SQL should not use postgres-only types: %s", sql)
	}
}

func TestSkillSpaceTableSQLForPostgresKeepsUniqueConstraint(t *testing.T) {
	sql := skillSpaceTableSQL("postgres")
	upper := strings.ToUpper(sql)

	if strings.Contains(upper, "UNIQUE INDEX") {
		t.Fatalf("postgres skill_spaces SQL should not use MySQL unique index syntax: %s", sql)
	}
	if !strings.Contains(upper, "CONSTRAINT IDX_TENANT_NAME_STATUS UNIQUE") {
		t.Fatalf("postgres skill_spaces SQL should enforce tenant/name/status uniqueness: %s", sql)
	}
}

func TestSkillSpaceTableSQLForMySQLKeepsUpstreamDialect(t *testing.T) {
	sql := skillSpaceTableSQL("mysql")
	upper := strings.ToUpper(sql)

	for _, required := range []string{
		"CREATE_DATE DATETIME",
		"INDEX IDX_TENANT_ID (TENANT_ID)",
		"UNIQUE INDEX IDX_TENANT_NAME_STATUS",
	} {
		if !strings.Contains(upper, required) {
			t.Fatalf("mysql skill_spaces SQL missing %q: %s", required, sql)
		}
	}
	if strings.Contains(upper, "CONSTRAINT IDX_TENANT_NAME_STATUS UNIQUE") {
		t.Fatalf("mysql skill_spaces SQL should keep upstream unique index syntax: %s", sql)
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

func TestUserEmailDuplicateCountSQLForPostgresQuotesUserTable(t *testing.T) {
	sql := userEmailDuplicateCountSQL("postgres")

	if !strings.Contains(sql, `FROM "user"`) {
		t.Fatalf("postgres duplicate email SQL should quote user table: %s", sql)
	}
}

func TestUserEmailDuplicateCountSQLForMySQLKeepsUserTableUnquoted(t *testing.T) {
	sql := userEmailDuplicateCountSQL("mysql")

	if !strings.Contains(sql, "FROM user") {
		t.Fatalf("mysql duplicate email SQL should keep user table unquoted: %s", sql)
	}
	if strings.Contains(sql, `"user"`) {
		t.Fatalf("mysql duplicate email SQL should not quote user table: %s", sql)
	}
}
