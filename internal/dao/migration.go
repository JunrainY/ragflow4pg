//
//  Copyright 2026 The InfiniFlow Authors. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package dao

import (
	"fmt"
	"ragflow/internal/common"
	"ragflow/internal/entity"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func isPostgres(db *gorm.DB) bool {
	return db.Dialector.Name() == "postgres"
}

// RunMigrations runs all manual database migrations
// These are migrations that cannot be handled by AutoMigrate alone
func RunMigrations(db *gorm.DB) error {
	// Check if tenant_llm table has composite primary key and migrate to ID primary key
	if err := migrateTenantLLMPrimaryKey(db); err != nil {
		return fmt.Errorf("failed to migrate tenant_llm primary key: %w", err)
	}

	// Rename columns (correct typos)
	if err := renameColumnIfExists(db, "task", "process_duation", "process_duration"); err != nil {
		return fmt.Errorf("failed to rename task.process_duation: %w", err)
	}
	if err := renameColumnIfExists(db, "document", "process_duation", "process_duration"); err != nil {
		return fmt.Errorf("failed to rename document.process_duation: %w", err)
	}

	// Add unique index on user.email
	if err := migrateAddUniqueEmail(db); err != nil {
		return fmt.Errorf("failed to add unique index on user.email: %w", err)
	}

	// Modify column types that AutoMigrate may not handle correctly
	if err := modifyColumnTypes(db); err != nil {
		return fmt.Errorf("failed to modify column types: %w", err)
	}

	// Create skill search tables
	if err := migrateSkillSearchTables(db); err != nil {
		return fmt.Errorf("failed to migrate skill search tables: %w", err)
	}

	// Create skill space tables
	if err := migrateSkillSpaceTables(db); err != nil {
		return fmt.Errorf("failed to migrate skill space tables: %w", err)
	}

	common.Info("All manual migrations completed successfully")
	return nil
}

// migrateTenantLLMPrimaryKey migrates tenant_llm from composite primary key to ID primary key
// This corresponds to Python's update_tenant_llm_to_id_primary_key function
func migrateTenantLLMPrimaryKey(db *gorm.DB) error {
	// Check if tenant_llm table exists
	if !db.Migrator().HasTable("tenant_llm") {
		return nil
	}

	if isPostgres(db) {
		return migrateTenantLLMPrimaryKeyPostgres(db)
	}

	// Check if 'id' column already exists using raw SQL
	var idColumnExists int64
	err := db.Raw(`
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_NAME = 'tenant_llm' AND COLUMN_NAME = 'id'
	`).Scan(&idColumnExists).Error
	if err != nil {
		return err
	}

	if idColumnExists > 0 {
		// Check if id is already a primary key with auto_increment
		var count int64
		err := db.Raw(`
			SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
			WHERE TABLE_NAME = 'tenant_llm' 
			AND COLUMN_NAME = 'id' 
			AND EXTRA LIKE '%auto_increment%'
		`).Scan(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			// Already migrated
			return nil
		}
	}

	common.Info("Migrating tenant_llm to use ID primary key...")

	// Start transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Check for temp_id column and drop it if exists
		var tempIdExists int64
		tx.Raw(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
			WHERE TABLE_NAME = 'tenant_llm' AND COLUMN_NAME = 'temp_id'`).Scan(&tempIdExists)
		if tempIdExists > 0 {
			if err := tx.Exec("ALTER TABLE tenant_llm DROP COLUMN temp_id").Error; err != nil {
				common.Warn("Failed to drop temp_id column", zap.Error(err))
			}
		}

		// Check if there's already an 'id' column
		if idColumnExists > 0 {
			// Modify existing id column to be auto_increment primary key
			if err := tx.Exec(`
				ALTER TABLE tenant_llm 
				MODIFY COLUMN id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY
			`).Error; err != nil {
				return fmt.Errorf("failed to modify id column: %w", err)
			}
		} else {
			// Add id column as auto_increment primary key
			if err := tx.Exec(`
				ALTER TABLE tenant_llm 
				ADD COLUMN id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY FIRST
			`).Error; err != nil {
				return fmt.Errorf("failed to add id column: %w", err)
			}
		}

		// Add unique index on (tenant_id, llm_factory, llm_name)
		var idxExists int64
		tx.Raw(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS 
			WHERE TABLE_NAME = 'tenant_llm' AND INDEX_NAME = 'idx_tenant_llm_unique'`).Scan(&idxExists)
		if idxExists == 0 {
			if err := tx.Exec(`
				ALTER TABLE tenant_llm 
				ADD UNIQUE INDEX idx_tenant_llm_unique (tenant_id, llm_factory, llm_name)
			`).Error; err != nil {
				common.Warn("Failed to add unique index idx_tenant_llm_unique", zap.Error(err))
			}
		}

		common.Info("tenant_llm primary key migration completed")
		return nil
	})
}

func migrateTenantLLMPrimaryKeyPostgres(db *gorm.DB) error {
	var idColumnExists int64
	err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_catalog = current_database()
		  AND table_name = 'tenant_llm'
		  AND column_name = 'id'
	`).Scan(&idColumnExists).Error
	if err != nil {
		return err
	}
	if idColumnExists > 0 {
		return nil
	}

	common.Info("Migrating tenant_llm to use ID primary key...")

	return db.Transaction(func(tx *gorm.DB) error {
		var tempIDExists int64
		tx.Raw(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_catalog = current_database()
			  AND table_name = 'tenant_llm'
			  AND column_name = 'temp_id'
		`).Scan(&tempIDExists)
		if tempIDExists > 0 {
			if err := tx.Exec("ALTER TABLE tenant_llm DROP COLUMN temp_id").Error; err != nil {
				common.Warn("Failed to drop temp_id column", zap.Error(err))
			}
		}

		if err := tx.Exec("ALTER TABLE tenant_llm ADD COLUMN temp_id BIGINT").Error; err != nil {
			return fmt.Errorf("failed to add temp_id column: %w", err)
		}

		if err := tx.Exec(`
			UPDATE tenant_llm
			SET temp_id = subq.rn
			FROM (
				SELECT ctid, ROW_NUMBER() OVER (ORDER BY tenant_id, llm_factory, llm_name) AS rn
				FROM tenant_llm
			) AS subq
			WHERE tenant_llm.ctid = subq.ctid
		`).Error; err != nil {
			return fmt.Errorf("failed to populate temp_id: %w", err)
		}

		var primaryKeyName string
		if err := tx.Raw(`
			SELECT constraint_name
			FROM information_schema.table_constraints
			WHERE table_catalog = current_database()
			  AND table_name = 'tenant_llm'
			  AND constraint_type = 'PRIMARY KEY'
			LIMIT 1
		`).Scan(&primaryKeyName).Error; err != nil {
			return err
		}
		if primaryKeyName != "" {
			if err := tx.Exec(fmt.Sprintf(`ALTER TABLE tenant_llm DROP CONSTRAINT "%s"`, primaryKeyName)).Error; err != nil {
				return fmt.Errorf("failed to drop existing primary key: %w", err)
			}
		}

		if err := tx.Exec("ALTER TABLE tenant_llm ALTER COLUMN temp_id SET NOT NULL").Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE SEQUENCE IF NOT EXISTS tenant_llm_id_seq").Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			SELECT setval('tenant_llm_id_seq', COALESCE((SELECT MAX(temp_id) FROM tenant_llm), 0))
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE tenant_llm ALTER COLUMN temp_id SET DEFAULT nextval('tenant_llm_id_seq')").Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER SEQUENCE tenant_llm_id_seq OWNED BY tenant_llm.temp_id").Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE tenant_llm ADD PRIMARY KEY (temp_id)").Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			ALTER TABLE tenant_llm
			ADD CONSTRAINT idx_tenant_llm_unique UNIQUE (tenant_id, llm_factory, llm_name)
		`).Error; err != nil {
			common.Warn("Failed to add unique constraint idx_tenant_llm_unique", zap.Error(err))
		}
		if err := tx.Exec("ALTER TABLE tenant_llm RENAME COLUMN temp_id TO id").Error; err != nil {
			return err
		}

		common.Info("tenant_llm primary key migration completed")
		return nil
	})
}

// migrateAddUniqueEmail adds unique index on user.email
func migrateAddUniqueEmail(db *gorm.DB) error {
	if !db.Migrator().HasTable("user") {
		return nil
	}

	if isPostgres(db) {
		var count int64
		db.Raw(`
			SELECT COUNT(*)
			FROM pg_indexes
			WHERE tablename = 'user'
			  AND indexname = 'idx_user_email_unique'
		`).Scan(&count)
		if count > 0 {
			return nil
		}
	} else {
		// Check if unique index already exists using raw SQL
		var count int64
		db.Raw(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS 
			WHERE TABLE_NAME = 'user' AND INDEX_NAME = 'idx_user_email_unique'`).Scan(&count)
		if count > 0 {
			return nil
		}
	}

	// Check if there's a duplicate email issue first
	var duplicateCount int64
	err := db.Raw(userEmailDuplicateCountSQL(db.Dialector.Name())).Scan(&duplicateCount).Error
	if err != nil {
		return err
	}

	if duplicateCount > 0 {
		common.Warn("Found duplicate emails in user table, cannot add unique index", zap.Int64("count", duplicateCount))
		return nil
	}

	common.Info("Adding unique index on user.email...")
	statement := `ALTER TABLE user ADD UNIQUE INDEX idx_user_email_unique (email)`
	if isPostgres(db) {
		statement = `CREATE UNIQUE INDEX idx_user_email_unique ON "user" (email)`
	}
	if err = db.Exec(statement).Error; err != nil {

		// Check if error is MySQL duplicate index error (Error 1061)
		errStr := err.Error()
		if (strings.Contains(errStr, "Error 1061") && strings.Contains(errStr, "Duplicate key name")) ||
			strings.Contains(strings.ToLower(errStr), "already exists") {
			common.Info("Index already exists, skipping", zap.String("error", errStr))
			return nil
		}
		return fmt.Errorf("failed to add unique index on email: %w", err)
	}

	return nil
}

func userEmailDuplicateCountSQL(driver string) string {
	userTable := "user"
	if driver == "postgres" {
		userTable = `"user"`
	}
	return fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT email FROM %s GROUP BY email HAVING COUNT(*) > 1
		) AS duplicates
	`, userTable)
}

// modifyColumnTypes modifies column types that need explicit ALTER statements
func modifyColumnTypes(db *gorm.DB) error {
	// Helper function to check if column exists
	columnExists := func(table, column string) bool {
		var count int64
		query := `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?`
		if isPostgres(db) {
			query = `SELECT COUNT(*) FROM information_schema.columns WHERE table_catalog = current_database() AND table_name = ? AND column_name = ?`
		}
		db.Raw(query, table, column).Scan(&count)
		return count > 0
	}

	// dialog.top_k: ensure it's INTEGER with default 1024
	if db.Migrator().HasTable("dialog") && columnExists("dialog", "top_k") {
		statement := `ALTER TABLE dialog MODIFY COLUMN top_k BIGINT NOT NULL DEFAULT 1024`
		if isPostgres(db) {
			statement = `ALTER TABLE dialog ALTER COLUMN top_k TYPE BIGINT`
		}
		if err := db.Exec(statement).Error; err != nil {
			common.Warn("Failed to modify dialog.top_k", zap.Error(err))
		}
	}

	// tenant_llm.api_key: ensure it's TEXT type
	if db.Migrator().HasTable("tenant_llm") && columnExists("tenant_llm", "api_key") {
		statement := `ALTER TABLE tenant_llm MODIFY COLUMN api_key LONGTEXT`
		if isPostgres(db) {
			statement = `ALTER TABLE tenant_llm ALTER COLUMN api_key TYPE TEXT`
		}
		if err := db.Exec(statement).Error; err != nil {
			common.Warn("Failed to modify tenant_llm.api_key", zap.Error(err))
		}
	}

	// api_token.dialog_id: ensure it's varchar(32)
	if db.Migrator().HasTable("api_token") && columnExists("api_token", "dialog_id") {
		statement := `ALTER TABLE api_token MODIFY COLUMN dialog_id VARCHAR(32)`
		if isPostgres(db) {
			statement = `ALTER TABLE api_token ALTER COLUMN dialog_id TYPE VARCHAR(32)`
		}
		if err := db.Exec(statement).Error; err != nil {
			common.Warn("Failed to modify api_token.dialog_id", zap.Error(err))
		}
	}

	// canvas_template.title and description: ensure they're LONGTEXT type (same as Python JSONField)
	// Note: Python's JSONField uses null=True with application-level default, not database DEFAULT
	if db.Migrator().HasTable("canvas_template") {
		if columnExists("canvas_template", "title") {
			statement := `ALTER TABLE canvas_template MODIFY COLUMN title LONGTEXT NULL`
			if isPostgres(db) {
				statement = `ALTER TABLE canvas_template ALTER COLUMN title TYPE TEXT`
			}
			if err := db.Exec(statement).Error; err != nil {
				common.Warn("Failed to modify canvas_template.title", zap.Error(err))
			}
		}
		if columnExists("canvas_template", "description") {
			statement := `ALTER TABLE canvas_template MODIFY COLUMN description LONGTEXT NULL`
			if isPostgres(db) {
				statement = `ALTER TABLE canvas_template ALTER COLUMN description TYPE TEXT`
			}
			if err := db.Exec(statement).Error; err != nil {
				common.Warn("Failed to modify canvas_template.description", zap.Error(err))
			}
		}
	}

	// system_settings.value: ensure it's LONGTEXT
	if db.Migrator().HasTable("system_settings") && columnExists("system_settings", "value") {
		statement := `ALTER TABLE system_settings MODIFY COLUMN value LONGTEXT NOT NULL`
		if isPostgres(db) {
			statement = `ALTER TABLE system_settings ALTER COLUMN value TYPE TEXT`
		}
		if err := db.Exec(statement).Error; err != nil {
			common.Warn("Failed to modify system_settings.value", zap.Error(err))
		}
	}

	// knowledgebase.raptor_task_finish_at: ensure it's DateTime
	if db.Migrator().HasTable("knowledgebase") && columnExists("knowledgebase", "raptor_task_finish_at") {
		statement := `ALTER TABLE knowledgebase MODIFY COLUMN raptor_task_finish_at DATETIME`
		if isPostgres(db) {
			statement = `ALTER TABLE knowledgebase ALTER COLUMN raptor_task_finish_at TYPE TIMESTAMP`
		}
		if err := db.Exec(statement).Error; err != nil {
			common.Warn("Failed to modify knowledgebase.raptor_task_finish_at", zap.Error(err))
		}
	}

	// knowledgebase.mindmap_task_finish_at: ensure it's DateTime
	if db.Migrator().HasTable("knowledgebase") && columnExists("knowledgebase", "mindmap_task_finish_at") {
		statement := `ALTER TABLE knowledgebase MODIFY COLUMN mindmap_task_finish_at DATETIME`
		if isPostgres(db) {
			statement = `ALTER TABLE knowledgebase ALTER COLUMN mindmap_task_finish_at TYPE TIMESTAMP`
		}
		if err := db.Exec(statement).Error; err != nil {
			common.Warn("Failed to modify knowledgebase.mindmap_task_finish_at", zap.Error(err))
		}
	}

	return nil
}

// renameColumnIfExists renames a column if it exists and the new column doesn't exist
func renameColumnIfExists(db *gorm.DB, tableName, oldName, newName string) error {
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	// Helper to check if column exists
	columnExists := func(column string) bool {
		var count int64
		query := `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?`
		if isPostgres(db) {
			query = `SELECT COUNT(*) FROM information_schema.columns WHERE table_catalog = current_database() AND table_name = ? AND column_name = ?`
		}
		db.Raw(query, tableName, column).Scan(&count)
		return count > 0
	}

	// Check if old column exists
	if !columnExists(oldName) {
		return nil
	}

	// Check if new column already exists
	if columnExists(newName) {
		// Both exist, drop the old one
		common.Warn("Both old and new columns exist, dropping old one",
			zap.String("table", tableName),
			zap.String("oldColumn", oldName),
			zap.String("newColumn", newName))
		return db.Migrator().DropColumn(tableName, oldName)
	}

	common.Info("Renaming column",
		zap.String("table", tableName),
		zap.String("oldColumn", oldName),
		zap.String("newColumn", newName))
	return db.Migrator().RenameColumn(tableName, oldName, newName)
}

// addColumnIfNotExists adds a column if it doesn't exist
func addColumnIfNotExists(db *gorm.DB, tableName, columnName, columnDef string) error {
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	// Check if column exists using raw SQL
	var count int64
	query := `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?`
	if isPostgres(db) {
		query = `SELECT COUNT(*) FROM information_schema.columns WHERE table_catalog = current_database() AND table_name = ? AND column_name = ?`
	}
	db.Raw(query, tableName, columnName).Scan(&count)
	if count > 0 {
		return nil
	}

	common.Info("Adding column",
		zap.String("table", tableName),
		zap.String("column", columnName))
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef)
	return db.Exec(sql).Error
}

func skillDateTimeColumnType(driver string) string {
	if driver == "postgres" {
		return "TIMESTAMP"
	}
	return "DATETIME"
}

func skillSearchTableSQL(driver string) string {
	if driver == "postgres" {
		return `
		CREATE TABLE IF NOT EXISTS skill_search_configs (
			id VARCHAR(32) PRIMARY KEY,
			tenant_id VARCHAR(32) NOT NULL,
			space_id VARCHAR(128) NOT NULL DEFAULT 'default',
			embd_id VARCHAR(128) NOT NULL,
			vector_similarity_weight DOUBLE PRECISION DEFAULT 0.3,
			similarity_threshold DOUBLE PRECISION DEFAULT 0.2,
			field_config JSONB,
			rerank_id VARCHAR(128),
			tenant_rerank_id BIGINT,
			top_k BIGINT DEFAULT 10,
			index_version VARCHAR(32) DEFAULT '1.0.0',
			status VARCHAR(1) DEFAULT '1',
			create_time BIGINT,
			create_date TIMESTAMP,
			update_time BIGINT,
			update_date TIMESTAMP,
			CONSTRAINT idx_tenant_space_embd UNIQUE (tenant_id, space_id, embd_id)
		)
		`
	}

	return `
		CREATE TABLE IF NOT EXISTS skill_search_configs (
			id VARCHAR(32) PRIMARY KEY,
			tenant_id VARCHAR(32) NOT NULL,
			space_id VARCHAR(128) NOT NULL DEFAULT 'default',
			embd_id VARCHAR(128) NOT NULL,
			vector_similarity_weight FLOAT DEFAULT 0.3,
			similarity_threshold FLOAT DEFAULT 0.2,
			field_config JSON,
			rerank_id VARCHAR(128),
			tenant_rerank_id BIGINT,
			top_k BIGINT DEFAULT 10,
			index_version VARCHAR(32) DEFAULT '1.0.0',
			status VARCHAR(1) DEFAULT '1',
			create_time BIGINT,
			create_date DATETIME,
			update_time BIGINT,
			update_date DATETIME,
			INDEX idx_tenant_id (tenant_id),
			INDEX idx_space_id (space_id),
			UNIQUE INDEX idx_tenant_space_embd (tenant_id, space_id, embd_id)
		)
		`
}

func skillSpaceTableSQL(driver string) string {
	if driver == "postgres" {
		return `
		CREATE TABLE IF NOT EXISTS skill_spaces (
			id VARCHAR(32) PRIMARY KEY,
			tenant_id VARCHAR(32) NOT NULL,
			name VARCHAR(128) NOT NULL,
			folder_id VARCHAR(32) NOT NULL,
			description TEXT,
			embd_id VARCHAR(128),
			rerank_id VARCHAR(128),
			top_k INT DEFAULT 10,
			status VARCHAR(1) DEFAULT '1',
			create_time BIGINT,
			create_date TIMESTAMP,
			update_time BIGINT,
			update_date TIMESTAMP,
			CONSTRAINT idx_tenant_name_status UNIQUE (tenant_id, name, status)
		)
		`
	}

	return `
		CREATE TABLE IF NOT EXISTS skill_spaces (
			id VARCHAR(32) PRIMARY KEY,
			tenant_id VARCHAR(32) NOT NULL,
			name VARCHAR(128) NOT NULL,
			folder_id VARCHAR(32) NOT NULL,
			description TEXT,
			embd_id VARCHAR(128),
			rerank_id VARCHAR(128),
			top_k INT DEFAULT 10,
			status VARCHAR(1) DEFAULT '1',
			create_time BIGINT,
			create_date DATETIME,
			update_time BIGINT,
			update_date DATETIME,
			INDEX idx_tenant_id (tenant_id),
			UNIQUE INDEX idx_tenant_name_status (tenant_id, name, status)
		)
		`
}

func indexExistsQuery(driver string) string {
	if driver == "postgres" {
		return `SELECT COUNT(*) FROM pg_indexes WHERE tablename = ? AND indexname = ?`
	}
	return `SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_NAME = ? AND INDEX_NAME = ?`
}

func legacySkillSearchIndexDropSQL(driver string) string {
	if driver == "postgres" {
		return `DROP INDEX IF EXISTS idx_tenant_embd`
	}
	return `ALTER TABLE skill_search_configs DROP INDEX idx_tenant_embd`
}

func skillSearchLookupIndexSQL(driver string) []string {
	if driver == "postgres" {
		return []string{
			`CREATE INDEX IF NOT EXISTS idx_skill_search_configs_tenant_id ON skill_search_configs(tenant_id)`,
			`CREATE INDEX IF NOT EXISTS idx_skill_search_configs_space_id ON skill_search_configs(space_id)`,
		}
	}
	return nil
}

func skillSearchUniqueIndexSQL(driver string) string {
	if driver == "postgres" {
		return `CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_space_embd ON skill_search_configs(tenant_id, space_id, embd_id)`
	}
	return `ALTER TABLE skill_search_configs ADD UNIQUE INDEX idx_tenant_space_embd (tenant_id, space_id, embd_id)`
}

func skillSpaceLookupIndexSQL(driver string) []string {
	if driver == "postgres" {
		return []string{
			`CREATE INDEX IF NOT EXISTS idx_skill_spaces_tenant_id ON skill_spaces(tenant_id)`,
		}
	}
	return nil
}

func skillSpaceUniqueIndexSQL(driver string) string {
	if driver == "postgres" {
		return `CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_name_status ON skill_spaces(tenant_id, name, status)`
	}
	return `ALTER TABLE skill_spaces ADD UNIQUE INDEX idx_tenant_name_status (tenant_id, name, status)`
}

func skillUpdateTimeColumnSQL(driver, table string) string {
	if driver == "postgres" {
		return fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN update_time TYPE BIGINT`, table)
	}
	return fmt.Sprintf(`ALTER TABLE %s MODIFY COLUMN update_time BIGINT`, table)
}

func skillSpaceIndexSQL(driver string) (string, string) {
	if driver == "postgres" {
		return `DROP INDEX IF EXISTS idx_tenant_name`, `CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_name_status ON skill_spaces(tenant_id, name, status)`
	}
	return `DROP INDEX idx_tenant_name ON skill_spaces`, `CREATE UNIQUE INDEX idx_tenant_name_status ON skill_spaces(tenant_id, name, status)`
}

func execSQLStatements(db *gorm.DB, statements []string) error {
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

// migrateSkillSearchTables creates skill search related tables
func migrateSkillSearchTables(db *gorm.DB) error {
	// Create skill_search_configs table only
	if !db.Migrator().HasTable("skill_search_configs") {
		common.Info("Creating skill_search_configs table...")
		sql := skillSearchTableSQL(db.Dialector.Name())
		if err := db.Exec(sql).Error; err != nil {
			common.Warn("Failed to create skill_search_configs table with dialect SQL, trying generic", zap.Error(err))
			if err := db.AutoMigrate(&entity.SkillSearchConfig{}); err != nil {
				return err
			}
			// AutoMigrate doesn't create unique indexes, so create them explicitly
			common.Info("Creating unique indexes for skill_search_configs...")
			if err := db.Exec(skillSearchUniqueIndexSQL(db.Dialector.Name())).Error; err != nil {
				return fmt.Errorf("failed to create unique index idx_tenant_space_embd: %w", err)
			}
		}
	} else {
		// Add space_id for existing installations.
		if err := addColumnIfNotExists(db, "skill_search_configs", "space_id", "VARCHAR(128) NOT NULL DEFAULT 'default'"); err != nil {
			return fmt.Errorf("failed to add space_id column to skill_search_configs: %w", err)
		}
		if err := addColumnIfNotExists(db, "skill_search_configs", "create_date", skillDateTimeColumnType(db.Dialector.Name())); err != nil {
			return fmt.Errorf("failed to add create_date column to skill_search_configs: %w", err)
		}
		if err := addColumnIfNotExists(db, "skill_search_configs", "update_date", skillDateTimeColumnType(db.Dialector.Name())); err != nil {
			return fmt.Errorf("failed to add update_date column to skill_search_configs: %w", err)
		}
		if err := db.Exec(skillUpdateTimeColumnSQL(db.Dialector.Name(), "skill_search_configs")).Error; err != nil {
			common.Warn("Failed to modify skill_search_configs.update_time", zap.Error(err))
		}

		// Drop legacy unique index (tenant_id, embd_id) to allow per-space configs.
		var legacyIndexExists int64
		db.Raw(indexExistsQuery(db.Dialector.Name()), "skill_search_configs", "idx_tenant_embd").Scan(&legacyIndexExists)
		if legacyIndexExists > 0 {
			common.Info("Dropping legacy unique index idx_tenant_embd from skill_search_configs...")
			if err := db.Exec(legacySkillSearchIndexDropSQL(db.Dialector.Name())).Error; err != nil {
				return fmt.Errorf("failed to drop legacy unique index idx_tenant_embd: %w", err)
			}
		}

		// Table exists, check if unique index exists
		var indexExists int64
		db.Raw(indexExistsQuery(db.Dialector.Name()), "skill_search_configs", "idx_tenant_space_embd").Scan(&indexExists)
		if indexExists == 0 {
			common.Info("Adding unique index idx_tenant_space_embd to skill_search_configs...")
			if err := db.Exec(skillSearchUniqueIndexSQL(db.Dialector.Name())).Error; err != nil {
				return fmt.Errorf("failed to add unique index idx_tenant_space_embd: %w", err)
			}
		}
	}

	if err := execSQLStatements(db, skillSearchLookupIndexSQL(db.Dialector.Name())); err != nil {
		return fmt.Errorf("failed to create lookup indexes for skill_search_configs: %w", err)
	}

	return nil
}

// migrateSkillSpaceTables creates skill space related tables
func migrateSkillSpaceTables(db *gorm.DB) error {
	if !db.Migrator().HasTable("skill_spaces") {
		common.Info("Creating skill_spaces table...")
		sql := skillSpaceTableSQL(db.Dialector.Name())
		if err := db.Exec(sql).Error; err != nil {
			common.Warn("Failed to create skill_spaces table with dialect SQL, trying generic", zap.Error(err))
			// Try with AutoMigrate as fallback
			if err := db.AutoMigrate(&entity.SkillSpace{}); err != nil {
				return err
			}
			// AutoMigrate doesn't create unique indexes, so create them explicitly
			common.Info("Creating unique indexes for skill_spaces...")
			if err := db.Exec(skillSpaceUniqueIndexSQL(db.Dialector.Name())).Error; err != nil {
				return fmt.Errorf("failed to create unique index idx_tenant_name_status: %w", err)
			}
		}
	} else {
		// Migrate existing table: add status column first, then update index
		if err := addColumnIfNotExists(db, "skill_spaces", "status", "VARCHAR(1) NOT NULL DEFAULT '1'"); err != nil {
			return fmt.Errorf("failed to add status column to skill_spaces: %w", err)
		}
		if err := addColumnIfNotExists(db, "skill_spaces", "create_date", skillDateTimeColumnType(db.Dialector.Name())); err != nil {
			return fmt.Errorf("failed to add create_date column to skill_spaces: %w", err)
		}
		if err := addColumnIfNotExists(db, "skill_spaces", "update_date", skillDateTimeColumnType(db.Dialector.Name())); err != nil {
			return fmt.Errorf("failed to add update_date column to skill_spaces: %w", err)
		}
		if err := db.Exec(skillUpdateTimeColumnSQL(db.Dialector.Name(), "skill_spaces")).Error; err != nil {
			common.Warn("Failed to modify skill_spaces.update_time", zap.Error(err))
		}
		// Migrate index after status column exists
		if err := migrateSkillSpaceIndex(db); err != nil {
			return fmt.Errorf("failed to migrate skill_space index: %w", err)
		}
	}

	if err := execSQLStatements(db, skillSpaceLookupIndexSQL(db.Dialector.Name())); err != nil {
		return fmt.Errorf("failed to create lookup indexes for skill_spaces: %w", err)
	}

	return nil
}

// migrateSkillSpaceIndex migrates the unique index to include status
func migrateSkillSpaceIndex(db *gorm.DB) error {
	// Check if old index exists and drop it
	var oldIndexExists int64
	db.Raw(indexExistsQuery(db.Dialector.Name()), "skill_spaces", "idx_tenant_name").Scan(&oldIndexExists)

	if oldIndexExists > 0 {
		common.Info("Dropping old idx_tenant_name index from skill_spaces...")
		dropSQL, _ := skillSpaceIndexSQL(db.Dialector.Name())
		if err := db.Exec(dropSQL).Error; err != nil {
			return fmt.Errorf("failed to drop old index idx_tenant_name: %w", err)
		}
	}

	// Check if new index exists
	var newIndexExists int64
	db.Raw(indexExistsQuery(db.Dialector.Name()), "skill_spaces", "idx_tenant_name_status").Scan(&newIndexExists)

	if newIndexExists == 0 {
		common.Info("Creating new idx_tenant_name_status index on skill_spaces...")
		_, createSQL := skillSpaceIndexSQL(db.Dialector.Name())
		if err := db.Exec(createSQL).Error; err != nil {
			return fmt.Errorf("failed to create unique index idx_tenant_name_status: %w", err)
		}
	}

	return nil
}
