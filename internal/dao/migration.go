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
	"ragflow/internal/logger"
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

	logger.Info("All manual migrations completed successfully")
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

	logger.Info("Migrating tenant_llm to use ID primary key...")

	// Start transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Check for temp_id column and drop it if exists
		var tempIdExists int64
		tx.Raw(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
			WHERE TABLE_NAME = 'tenant_llm' AND COLUMN_NAME = 'temp_id'`).Scan(&tempIdExists)
		if tempIdExists > 0 {
			if err := tx.Exec("ALTER TABLE tenant_llm DROP COLUMN temp_id").Error; err != nil {
				logger.Warn("Failed to drop temp_id column", zap.Error(err))
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
				logger.Warn("Failed to add unique index idx_tenant_llm_unique", zap.Error(err))
			}
		}

		logger.Info("tenant_llm primary key migration completed")
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

	logger.Info("Migrating tenant_llm to use ID primary key...")

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
				logger.Warn("Failed to drop temp_id column", zap.Error(err))
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
			logger.Warn("Failed to add unique constraint idx_tenant_llm_unique", zap.Error(err))
		}
		if err := tx.Exec("ALTER TABLE tenant_llm RENAME COLUMN temp_id TO id").Error; err != nil {
			return err
		}

		logger.Info("tenant_llm primary key migration completed")
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
	err := db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT email FROM user GROUP BY email HAVING COUNT(*) > 1
		) AS duplicates
	`).Scan(&duplicateCount).Error
	if err != nil {
		return err
	}

	if duplicateCount > 0 {
		logger.Warn("Found duplicate emails in user table, cannot add unique index", zap.Int64("count", duplicateCount))
		return nil
	}

	logger.Info("Adding unique index on user.email...")
	statement := `ALTER TABLE user ADD UNIQUE INDEX idx_user_email_unique (email)`
	if isPostgres(db) {
		statement = `CREATE UNIQUE INDEX idx_user_email_unique ON "user" (email)`
	}
	if err = db.Exec(statement).Error; err != nil {

		// Check if error is MySQL duplicate index error (Error 1061)
		errStr := err.Error()
		if (strings.Contains(errStr, "Error 1061") && strings.Contains(errStr, "Duplicate key name")) ||
			strings.Contains(strings.ToLower(errStr), "already exists") {
			logger.Info("Index already exists, skipping", zap.String("error", errStr))
			return nil
		}
		return fmt.Errorf("failed to add unique index on email: %w", err)
	}

	return nil
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
			logger.Warn("Failed to modify dialog.top_k", zap.Error(err))
		}
	}

	// tenant_llm.api_key: ensure it's TEXT type
	if db.Migrator().HasTable("tenant_llm") && columnExists("tenant_llm", "api_key") {
		statement := `ALTER TABLE tenant_llm MODIFY COLUMN api_key LONGTEXT`
		if isPostgres(db) {
			statement = `ALTER TABLE tenant_llm ALTER COLUMN api_key TYPE TEXT`
		}
		if err := db.Exec(statement).Error; err != nil {
			logger.Warn("Failed to modify tenant_llm.api_key", zap.Error(err))
		}
	}

	// api_token.dialog_id: ensure it's varchar(32)
	if db.Migrator().HasTable("api_token") && columnExists("api_token", "dialog_id") {
		statement := `ALTER TABLE api_token MODIFY COLUMN dialog_id VARCHAR(32)`
		if isPostgres(db) {
			statement = `ALTER TABLE api_token ALTER COLUMN dialog_id TYPE VARCHAR(32)`
		}
		if err := db.Exec(statement).Error; err != nil {
			logger.Warn("Failed to modify api_token.dialog_id", zap.Error(err))
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
				logger.Warn("Failed to modify canvas_template.title", zap.Error(err))
			}
		}
		if columnExists("canvas_template", "description") {
			statement := `ALTER TABLE canvas_template MODIFY COLUMN description LONGTEXT NULL`
			if isPostgres(db) {
				statement = `ALTER TABLE canvas_template ALTER COLUMN description TYPE TEXT`
			}
			if err := db.Exec(statement).Error; err != nil {
				logger.Warn("Failed to modify canvas_template.description", zap.Error(err))
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
			logger.Warn("Failed to modify system_settings.value", zap.Error(err))
		}
	}

	// knowledgebase.raptor_task_finish_at: ensure it's DateTime
	if db.Migrator().HasTable("knowledgebase") && columnExists("knowledgebase", "raptor_task_finish_at") {
		statement := `ALTER TABLE knowledgebase MODIFY COLUMN raptor_task_finish_at DATETIME`
		if isPostgres(db) {
			statement = `ALTER TABLE knowledgebase ALTER COLUMN raptor_task_finish_at TYPE TIMESTAMP`
		}
		if err := db.Exec(statement).Error; err != nil {
			logger.Warn("Failed to modify knowledgebase.raptor_task_finish_at", zap.Error(err))
		}
	}

	// knowledgebase.mindmap_task_finish_at: ensure it's DateTime
	if db.Migrator().HasTable("knowledgebase") && columnExists("knowledgebase", "mindmap_task_finish_at") {
		statement := `ALTER TABLE knowledgebase MODIFY COLUMN mindmap_task_finish_at DATETIME`
		if isPostgres(db) {
			statement = `ALTER TABLE knowledgebase ALTER COLUMN mindmap_task_finish_at TYPE TIMESTAMP`
		}
		if err := db.Exec(statement).Error; err != nil {
			logger.Warn("Failed to modify knowledgebase.mindmap_task_finish_at", zap.Error(err))
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
		logger.Warn("Both old and new columns exist, dropping old one",
			zap.String("table", tableName),
			zap.String("oldColumn", oldName),
			zap.String("newColumn", newName))
		return db.Migrator().DropColumn(tableName, oldName)
	}

	logger.Info("Renaming column",
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

	logger.Info("Adding column",
		zap.String("table", tableName),
		zap.String("column", columnName))
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef)
	return db.Exec(sql).Error
}
