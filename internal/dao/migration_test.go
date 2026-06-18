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

func TestSkillSearchLookupIndexSQLForPostgresUsesPostgresDialect(t *testing.T) {
	statements := skillSearchLookupIndexSQL("postgres")
	if len(statements) != 2 {
		t.Fatalf("postgres skill_search_configs should create two lookup indexes, got %d: %v", len(statements), statements)
	}

	joined := strings.ToUpper(strings.Join(statements, "\n"))
	for _, forbidden := range []string{"ALTER TABLE", " ADD INDEX ", " ADD UNIQUE INDEX ", "IDX_TENANT_ID ON"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("postgres skill_search_configs index SQL contains invalid or conflicting syntax %q: %s", forbidden, joined)
		}
	}
	for _, required := range []string{
		"CREATE INDEX IF NOT EXISTS IDX_SKILL_SEARCH_CONFIGS_TENANT_ID ON SKILL_SEARCH_CONFIGS(TENANT_ID)",
		"CREATE INDEX IF NOT EXISTS IDX_SKILL_SEARCH_CONFIGS_SPACE_ID ON SKILL_SEARCH_CONFIGS(SPACE_ID)",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("postgres skill_search_configs index SQL missing %q: %s", required, joined)
		}
	}
}

func TestSkillSearchLookupIndexSQLForMySQLDoesNotDuplicateInlineIndexes(t *testing.T) {
	if statements := skillSearchLookupIndexSQL("mysql"); len(statements) != 0 {
		t.Fatalf("mysql skill_search_configs should keep lookup indexes inline, got extra SQL: %v", statements)
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

func TestSkillSpaceLookupIndexSQLForPostgresUsesPostgresDialect(t *testing.T) {
	statements := skillSpaceLookupIndexSQL("postgres")
	if len(statements) != 1 {
		t.Fatalf("postgres skill_spaces should create one lookup index, got %d: %v", len(statements), statements)
	}

	upper := strings.ToUpper(statements[0])
	for _, forbidden := range []string{"ALTER TABLE", " ADD INDEX ", " ADD UNIQUE INDEX ", "IDX_TENANT_ID ON"} {
		if strings.Contains(upper, forbidden) {
			t.Fatalf("postgres skill_spaces index SQL contains invalid or conflicting syntax %q: %s", forbidden, statements[0])
		}
	}
	required := "CREATE INDEX IF NOT EXISTS IDX_SKILL_SPACES_TENANT_ID ON SKILL_SPACES(TENANT_ID)"
	if !strings.Contains(upper, required) {
		t.Fatalf("postgres skill_spaces index SQL missing %q: %s", required, statements[0])
	}
}

func TestSkillSpaceLookupIndexSQLForMySQLDoesNotDuplicateInlineIndexes(t *testing.T) {
	if statements := skillSpaceLookupIndexSQL("mysql"); len(statements) != 0 {
		t.Fatalf("mysql skill_spaces should keep lookup indexes inline, got extra SQL: %v", statements)
	}
}

func TestSkillSpaceIndexSQLForPostgresUsesPostgresDialect(t *testing.T) {
	dropSQL, createSQL := skillSpaceIndexSQL("postgres")

	if strings.Contains(strings.ToUpper(dropSQL), " ON ") {
		t.Fatalf("postgres drop index SQL should not use MySQL ON clause: %s", dropSQL)
	}
	if !strings.Contains(strings.ToUpper(createSQL), "CREATE UNIQUE INDEX IF NOT EXISTS") {
		t.Fatalf("postgres create index SQL should create an idempotent unique index: %s", createSQL)
	}
}

func TestSkillUniqueIndexSQLForPostgresIsIdempotent(t *testing.T) {
	for _, sql := range []string{
		skillSearchUniqueIndexSQL("postgres"),
		skillSpaceUniqueIndexSQL("postgres"),
	} {
		upper := strings.ToUpper(sql)
		if strings.Contains(upper, "ALTER TABLE") {
			t.Fatalf("postgres unique index SQL should not use ALTER TABLE: %s", sql)
		}
		if !strings.Contains(upper, "CREATE UNIQUE INDEX IF NOT EXISTS") {
			t.Fatalf("postgres unique index SQL should be idempotent: %s", sql)
		}
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
