package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================================
// EAMSA 512 - Database Layer
// Persistence layer for encryption operations, audit logs, and key metadata
//
// Uses SQLite3 for simplicity and portability, with migrations for schema management.
// Implements comprehensive audit logging and operational tracking.
//
// Last updated: December 4, 2025
// ============================================================================

// Database represents the EAMSA 512 database connection
type Database struct {
	conn       *sql.DB
	mu         sync.RWMutex
	logger     *log.Logger
	dbPath     string
	maxRetries int
}

// OperationRecord represents a single encryption/decryption operation
type OperationRecord struct {
	ID              int64      `json:"id"`
	OperationType   string     `json:"operation_type"`   // "encrypt" or "decrypt"
	KeyVersion      int        `json:"key_version"`      // Which key was used
	PlaintextSize   int        `json:"plaintext_size"`   // Size of plaintext
	CiphertextSize  int        `json:"ciphertext_size"`  // Size of ciphertext
	Timestamp       time.Time  `json:"timestamp"`        // Operation time
	Status          string     `json:"status"`           // "success" or "failed"
	ErrorMessage    string     `json:"error_message"`    // Error details if failed
	ClientIP        string     `json:"client_ip"`        // Client IP address
	UserID          string     `json:"user_id"`          // Authenticated user (if available)
	RequestID       string     `json:"request_id"`       // Unique request identifier
	DurationMS      int64      `json:"duration_ms"`      // Operation duration in milliseconds
}

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	ID        int64      `json:"id"`
	EventType string     `json:"event_type"`  // "KEY_CREATED", "KEY_ROTATED", "LOGIN", etc.
	Category  string     `json:"category"`    // "security", "operation", "system", "admin"
	Severity  string     `json:"severity"`    // "info", "warning", "critical"
	Details   string     `json:"details"`     // JSON-encoded event details
	Timestamp time.Time  `json:"timestamp"`   // Event time
	UserID    string     `json:"user_id"`     // Acting user
	SourceIP  string     `json:"source_ip"`   // Source IP address
}

// KeyVersionRecord represents a stored key version record
type KeyVersionRecord struct {
	ID              int64      `json:"id"`
	Version         int        `json:"version"`
	State           string     `json:"state"`
	KeyHash         string     `json:"key_hash"`
	CreatedAt       time.Time  `json:"created_at"`
	ActivatedAt     time.Time  `json:"activated_at"`
	RotatedAt       time.Time  `json:"rotated_at"`
	EncryptionCount int64      `json:"encryption_count"`
	DecryptionCount int64      `json:"decryption_count"`
}

// ComplianceMetrics represents compliance-related metrics
type ComplianceMetrics struct {
	TotalEncryptions     int64     `json:"total_encryptions"`
	TotalDecryptions     int64     `json:"total_decryptions"`
	FailedOperations     int64     `json:"failed_operations"`
	KeyRotations         int64     `json:"key_rotations"`
	SecurityEvents       int64     `json:"security_events"`
	UnauthorizedAttempts int64     `json:"unauthorized_attempts"`
	AverageDurationMS    float64   `json:"average_duration_ms"`
	Timestamp            time.Time `json:"timestamp"`
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	// Create logger
	logFile, err := os.OpenFile("/var/log/eamsa512/database.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open database log: %v", err)
	}

	logger := log.New(logFile, "[DATABASE] ", log.LstdFlags|log.Lshortfile)

	// Open SQLite connection
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Set connection pool settings
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	db := &Database{
		conn:       conn,
		logger:     logger,
		dbPath:     dbPath,
		maxRetries: 3,
	}

	// Run migrations
	if err := db.runMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	logger.Printf("Database initialized at %s", dbPath)
	return db, nil
}

// runMigrations creates necessary tables if they don't exist
func (db *Database) runMigrations() error {
	// Create tables
	schemas := []string{
		// Operations table
		`CREATE TABLE IF NOT EXISTS operations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_type TEXT NOT NULL,
			key_version INTEGER NOT NULL,
			plaintext_size INTEGER,
			ciphertext_size INTEGER,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT NOT NULL,
			error_message TEXT,
			client_ip TEXT,
			user_id TEXT,
			request_id TEXT UNIQUE,
			duration_ms INTEGER,
			FOREIGN KEY(key_version) REFERENCES key_versions(version)
		)`,

		// Audit log table
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type TEXT NOT NULL,
			category TEXT NOT NULL,
			severity TEXT NOT NULL,
			details TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			user_id TEXT,
			source_ip TEXT
		)`,

		// Key versions table
		`CREATE TABLE IF NOT EXISTS key_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER UNIQUE NOT NULL,
			state TEXT NOT NULL,
			key_hash TEXT,
			created_at DATETIME,
			activated_at DATETIME,
			rotated_at DATETIME,
			encryption_count INTEGER DEFAULT 0,
			decryption_count INTEGER DEFAULT 0
		)`,

		// Sessions table
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT UNIQUE NOT NULL,
			user_id TEXT NOT NULL,
			ip_address TEXT,
			user_agent TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_activity DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			is_active BOOLEAN DEFAULT 1
		)`,

		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT UNIQUE NOT NULL,
			username TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME,
			is_active BOOLEAN DEFAULT 1
		)`,
	}

	for _, schema := range schemas {
		if _, err := db.conn.Exec(schema); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	// Create indexes for performance
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_operations_timestamp ON operations(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_operations_key_version ON operations(key_version)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_category ON audit_logs(category)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_key_versions_state ON key_versions(state)`,
	}

	for _, idx := range indexes {
		if _, err := db.conn.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %v", err)
		}
	}

	db.logger.Printf("Migrations completed successfully")
	return nil
}

// ============================================================================
// Operation Recording
// ============================================================================

// RecordOperation records an encryption or decryption operation
func (db *Database) RecordOperation(op OperationRecord) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO operations 
		(operation_type, key_version, plaintext_size, ciphertext_size, 
		 timestamp, status, error_message, client_ip, user_id, request_id, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query,
		op.OperationType, op.KeyVersion, op.PlaintextSize, op.CiphertextSize,
		op.Timestamp, op.Status, op.ErrorMessage, op.ClientIP, op.UserID,
		op.RequestID, op.DurationMS)

	if err != nil {
		db.logger.Printf("Failed to record operation: %v", err)
		return fmt.Errorf("failed to record operation: %v", err)
	}

	id, _ := result.LastInsertId()
	db.logger.Printf("Operation recorded: id=%d type=%s status=%s", id, op.OperationType, op.Status)
	return nil
}

// GetOperations retrieves recent operations with optional filtering
func (db *Database) GetOperations(limit int, offset int) ([]OperationRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, operation_type, key_version, plaintext_size, ciphertext_size,
		         timestamp, status, error_message, client_ip, user_id, request_id, duration_ms
		 FROM operations
		 ORDER BY timestamp DESC
		 LIMIT ? OFFSET ?`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query operations: %v", err)
	}
	defer rows.Close()

	operations := make([]OperationRecord, 0)
	for rows.Next() {
		var op OperationRecord
		err := rows.Scan(&op.ID, &op.OperationType, &op.KeyVersion, &op.PlaintextSize,
			&op.CiphertextSize, &op.Timestamp, &op.Status, &op.ErrorMessage,
			&op.ClientIP, &op.UserID, &op.RequestID, &op.DurationMS)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation: %v", err)
		}
		operations = append(operations, op)
	}

	return operations, nil
}

// GetOperationsByKeyVersion retrieves operations for a specific key version
func (db *Database) GetOperationsByKeyVersion(keyVersion int) ([]OperationRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, operation_type, key_version, plaintext_size, ciphertext_size,
		         timestamp, status, error_message, client_ip, user_id, request_id, duration_ms
		 FROM operations
		 WHERE key_version = ?
		 ORDER BY timestamp DESC`

	rows, err := db.conn.Query(query, keyVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to query operations: %v", err)
	}
	defer rows.Close()

	operations := make([]OperationRecord, 0)
	for rows.Next() {
		var op OperationRecord
		err := rows.Scan(&op.ID, &op.OperationType, &op.KeyVersion, &op.PlaintextSize,
			&op.CiphertextSize, &op.Timestamp, &op.Status, &op.ErrorMessage,
			&op.ClientIP, &op.UserID, &op.RequestID, &op.DurationMS)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation: %v", err)
		}
		operations = append(operations, op)
	}

	return operations, nil
}

// ============================================================================
// Audit Logging
// ============================================================================

// RecordAuditLog records an audit event
func (db *Database) RecordAuditLog(entry AuditLogEntry) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO audit_logs 
		(event_type, category, severity, details, timestamp, user_id, source_ip)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query,
		entry.EventType, entry.Category, entry.Severity, entry.Details,
		entry.Timestamp, entry.UserID, entry.SourceIP)

	if err != nil {
		db.logger.Printf("Failed to record audit log: %v", err)
		return fmt.Errorf("failed to record audit log: %v", err)
	}

	id, _ := result.LastInsertId()
	db.logger.Printf("Audit log recorded: id=%d event=%s severity=%s", id, entry.EventType, entry.Severity)
	return nil
}

// GetAuditLogs retrieves recent audit log entries
func (db *Database) GetAuditLogs(limit int, offset int) ([]AuditLogEntry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, event_type, category, severity, details, timestamp, user_id, source_ip
		 FROM audit_logs
		 ORDER BY timestamp DESC
		 LIMIT ? OFFSET ?`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %v", err)
	}
	defer rows.Close()

	logs := make([]AuditLogEntry, 0)
	for rows.Next() {
		var log AuditLogEntry
		err := rows.Scan(&log.ID, &log.EventType, &log.Category, &log.Severity,
			&log.Details, &log.Timestamp, &log.UserID, &log.SourceIP)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %v", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetAuditLogsByCategory retrieves audit logs by category
func (db *Database) GetAuditLogsByCategory(category string, limit int) ([]AuditLogEntry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, event_type, category, severity, details, timestamp, user_id, source_ip
		 FROM audit_logs
		 WHERE category = ?
		 ORDER BY timestamp DESC
		 LIMIT ?`

	rows, err := db.conn.Query(query, category, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %v", err)
	}
	defer rows.Close()

	logs := make([]AuditLogEntry, 0)
	for rows.Next() {
		var log AuditLogEntry
		err := rows.Scan(&log.ID, &log.EventType, &log.Category, &log.Severity,
			&log.Details, &log.Timestamp, &log.UserID, &log.SourceIP)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %v", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// ============================================================================
// Key Version Tracking
// ============================================================================

// RecordKeyVersion records a new key version
func (db *Database) RecordKeyVersion(kvr KeyVersionRecord) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT OR REPLACE INTO key_versions 
		(version, state, key_hash, created_at, activated_at, rotated_at, 
		 encryption_count, decryption_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query,
		kvr.Version, kvr.State, kvr.KeyHash, kvr.CreatedAt, kvr.ActivatedAt,
		kvr.RotatedAt, kvr.EncryptionCount, kvr.DecryptionCount)

	if err != nil {
		db.logger.Printf("Failed to record key version: %v", err)
		return fmt.Errorf("failed to record key version: %v", err)
	}

	id, _ := result.LastInsertId()
	db.logger.Printf("Key version recorded: id=%d version=%d state=%s", id, kvr.Version, kvr.State)
	return nil
}

// GetKeyVersions retrieves all key versions
func (db *Database) GetKeyVersions() ([]KeyVersionRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, version, state, key_hash, created_at, activated_at, 
		         rotated_at, encryption_count, decryption_count
		 FROM key_versions
		 ORDER BY version DESC`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query key versions: %v", err)
	}
	defer rows.Close()

	versions := make([]KeyVersionRecord, 0)
	for rows.Next() {
		var kvr KeyVersionRecord
		err := rows.Scan(&kvr.ID, &kvr.Version, &kvr.State, &kvr.KeyHash,
			&kvr.CreatedAt, &kvr.ActivatedAt, &kvr.RotatedAt,
			&kvr.EncryptionCount, &kvr.DecryptionCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan key version: %v", err)
		}
		versions = append(versions, kvr)
	}

	return versions, nil
}

// GetActiveKeyVersion retrieves the active key version
func (db *Database) GetActiveKeyVersion() (*KeyVersionRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, version, state, key_hash, created_at, activated_at,
		         rotated_at, encryption_count, decryption_count
		 FROM key_versions
		 WHERE state = 'active'
		 ORDER BY version DESC
		 LIMIT 1`

	var kvr KeyVersionRecord
	err := db.conn.QueryRow(query).Scan(&kvr.ID, &kvr.Version, &kvr.State, &kvr.KeyHash,
		&kvr.CreatedAt, &kvr.ActivatedAt, &kvr.RotatedAt,
		&kvr.EncryptionCount, &kvr.DecryptionCount)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query active key version: %v", err)
	}

	return &kvr, nil
}

// UpdateKeyVersionCounts updates encryption/decryption counts
func (db *Database) UpdateKeyVersionCounts(version int, encCount, decCount int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `UPDATE key_versions 
		 SET encryption_count = ?, decryption_count = ?
		 WHERE version = ?`

	_, err := db.conn.Exec(query, encCount, decCount, version)
	if err != nil {
		return fmt.Errorf("failed to update key version counts: %v", err)
	}

	return nil
}

// ============================================================================
// Compliance Metrics
// ============================================================================

// GetComplianceMetrics calculates and returns compliance metrics
func (db *Database) GetComplianceMetrics() (ComplianceMetrics, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	metrics := ComplianceMetrics{
		Timestamp: time.Now(),
	}

	// Count operations
	query := `SELECT 
		SUM(CASE WHEN operation_type = 'encrypt' THEN 1 ELSE 0 END) as encryptions,
		SUM(CASE WHEN operation_type = 'decrypt' THEN 1 ELSE 0 END) as decryptions,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failures,
		AVG(CASE WHEN duration_ms > 0 THEN duration_ms ELSE NULL END) as avg_duration
		FROM operations`

	err := db.conn.QueryRow(query).Scan(&metrics.TotalEncryptions, &metrics.TotalDecryptions,
		&metrics.FailedOperations, &metrics.AverageDurationMS)
	if err != nil && err != sql.ErrNoRows {
		return metrics, fmt.Errorf("failed to query metrics: %v", err)
	}

	// Count audit events
	auditQuery := `SELECT 
		SUM(CASE WHEN event_type LIKE 'KEY_%' THEN 1 ELSE 0 END) as rotations,
		SUM(CASE WHEN category = 'security' THEN 1 ELSE 0 END) as security_events,
		SUM(CASE WHEN severity = 'critical' THEN 1 ELSE 0 END) as unauthorized
		FROM audit_logs`

	err = db.conn.QueryRow(auditQuery).Scan(&metrics.KeyRotations, &metrics.SecurityEvents,
		&metrics.UnauthorizedAttempts)
	if err != nil && err != sql.ErrNoRows {
		return metrics, fmt.Errorf("failed to query audit metrics: %v", err)
	}

	return metrics, nil
}

// ============================================================================
// Session Management
// ============================================================================

// CreateSession creates a new session
func (db *Database) CreateSession(sessionID, userID, ipAddress, userAgent string, expiresAt time.Time) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO sessions 
		(session_id, user_id, ip_address, user_agent, expires_at)
		VALUES (?, ?, ?, ?, ?)`

	_, err := db.conn.Exec(query, sessionID, userID, ipAddress, userAgent, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}

	db.logger.Printf("Session created: sessionID=%s userID=%s", sessionID, userID)
	return nil
}

// ValidateSession validates an active session
func (db *Database) ValidateSession(sessionID string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT user_id FROM sessions 
		 WHERE session_id = ? AND is_active = 1 AND expires_at > datetime('now')`

	var userID string
	err := db.conn.QueryRow(query, sessionID).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("invalid or expired session")
	}
	if err != nil {
		return "", fmt.Errorf("failed to validate session: %v", err)
	}

	// Update last activity
	updateQuery := `UPDATE sessions SET last_activity = datetime('now') WHERE session_id = ?`
	db.conn.Exec(updateQuery, sessionID)

	return userID, nil
}

// EndSession terminates a session
func (db *Database) EndSession(sessionID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `UPDATE sessions SET is_active = 0 WHERE session_id = ?`
	_, err := db.conn.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to end session: %v", err)
	}

	db.logger.Printf("Session ended: sessionID=%s", sessionID)
	return nil
}

// ============================================================================
// Maintenance and Cleanup
// ============================================================================

// PruneOldRecords removes old operation and audit log records
func (db *Database) PruneOldRecords(daysToKeep int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep)

	// Delete old operations
	query1 := `DELETE FROM operations WHERE timestamp < ?`
	result1, err := db.conn.Exec(query1, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to prune operations: %v", err)
	}

	deleted1, _ := result1.RowsAffected()

	// Delete old audit logs
	query2 := `DELETE FROM audit_logs WHERE timestamp < ?`
	result2, err := db.conn.Exec(query2, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to prune audit logs: %v", err)
	}

	deleted2, _ := result2.RowsAffected()

	db.logger.Printf("Pruned records: operations=%d auditLogs=%d cutoffDate=%s",
		deleted1, deleted2, cutoffDate.Format(time.RFC3339))

	return nil
}

// Vacuum optimizes the database
func (db *Database) Vacuum() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %v", err)
	}

	db.logger.Printf("Database vacuumed")
	return nil
}

// Close closes the database connection
func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.conn != nil {
		err := db.conn.Close()
		db.logger.Printf("Database connection closed")
		return err
	}

	return nil
}

// ============================================================================
// Export Functions
// ============================================================================

// ExportOperationsJSON exports operations as JSON
func (db *Database) ExportOperationsJSON(limit int) (string, error) {
	ops, err := db.GetOperations(limit, 0)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return string(data), nil
}

// ExportAuditLogsJSON exports audit logs as JSON
func (db *Database) ExportAuditLogsJSON(limit int) (string, error) {
	logs, err := db.GetAuditLogs(limit, 0)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return string(data), nil
}

// ============================================================================
// Example Usage and Testing
// ============================================================================

func main() {
	fmt.Println("EAMSA 512 - Database Layer")
	fmt.Println("===========================\n")

	// Initialize database
	db, err := NewDatabase("/tmp/eamsa512.db")
	if err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}
	defer db.Close()

	fmt.Println("Database initialized successfully\n")

	// Record a key version
	fmt.Println("Recording key version...")
	kvr := KeyVersionRecord{
		Version:     1,
		State:       "active",
		KeyHash:     "abc123def456",
		CreatedAt:   time.Now(),
		ActivatedAt: time.Now(),
	}

	if err := db.RecordKeyVersion(kvr); err != nil {
		fmt.Printf("Error recording key version: %v\n", err)
		return
	}

	// Record operations
	fmt.Println("Recording operations...")
	for i := 0; i < 3; i++ {
		op := OperationRecord{
			OperationType:  "encrypt",
			KeyVersion:     1,
			PlaintextSize:  100,
			CiphertextSize: 144,
			Timestamp:      time.Now(),
			Status:         "success",
			ClientIP:       "192.168.1.100",
			UserID:         "user1",
			RequestID:      fmt.Sprintf("req_%d", i),
			DurationMS:     int64(5 + i),
		}

		if err := db.RecordOperation(op); err != nil {
			fmt.Printf("Error recording operation: %v\n", err)
			return
		}
	}

	// Record audit logs
	fmt.Println("Recording audit logs...")
	entry := AuditLogEntry{
		EventType: "KEY_CREATED",
		Category:  "security",
		Severity:  "info",
		Details:   `{"version": 1, "hash": "abc123def456"}`,
		Timestamp: time.Now(),
		UserID:    "admin",
		SourceIP:  "192.168.1.50",
	}

	if err := db.RecordAuditLog(entry); err != nil {
		fmt.Printf("Error recording audit log: %v\n", err)
		return
	}

	// Retrieve and display data
	fmt.Println("\nRetrieving operations...")
	ops, err := db.GetOperations(10, 0)
	if err != nil {
		fmt.Printf("Error retrieving operations: %v\n", err)
		return
	}

	for _, op := range ops {
		fmt.Printf("  Op: type=%s keyVer=%d status=%s duration=%dms\n",
			op.OperationType, op.KeyVersion, op.Status, op.DurationMS)
	}

	fmt.Println("\nRetrieving audit logs...")
	logs, err := db.GetAuditLogs(10, 0)
	if err != nil {
		fmt.Printf("Error retrieving audit logs: %v\n", err)
		return
	}

	for _, log := range logs {
		fmt.Printf("  Log: event=%s category=%s severity=%s\n",
			log.EventType, log.Category, log.Severity)
	}

	fmt.Println("\nRetrieving key versions...")
	versions, err := db.GetKeyVersions()
	if err != nil {
		fmt.Printf("Error retrieving key versions: %v\n", err)
		return
	}

	for _, v := range versions {
		fmt.Printf("  Version: %d state=%s hash=%s\n", v.Version, v.State, v.KeyHash)
	}

	// Get compliance metrics
	fmt.Println("\nCalculating compliance metrics...")
	metrics, err := db.GetComplianceMetrics()
	if err != nil {
		fmt.Printf("Error calculating metrics: %v\n", err)
		return
	}

	fmt.Printf("  Total Encryptions: %d\n", metrics.TotalEncryptions)
	fmt.Printf("  Total Decryptions: %d\n", metrics.TotalDecryptions)
	fmt.Printf("  Failed Operations: %d\n", metrics.FailedOperations)
	fmt.Printf("  Average Duration: %.2fms\n", metrics.AverageDurationMS)

	fmt.Println("\n✓ Database layer test completed successfully")
}

// ============================================================================
// NOTES
// ============================================================================

/*

1. DATABASE SCHEMA
   - operations: Records all encrypt/decrypt operations
   - audit_logs: Security and system events
   - key_versions: Key lifecycle tracking
   - sessions: User session management
   - users: User account information

2. INDEXES
   - Created on timestamp, key_version, category for performance
   - Enables efficient querying of large datasets

3. THREAD SAFETY
   - RWMutex protects all database operations
   - Multiple readers, single writer pattern

4. AUDIT TRAIL
   - Complete record of all cryptographic operations
   - Timestamps for all events
   - User and IP tracking
   - Operation status and duration

5. COMPLIANCE
   - FIPS 140-2 audit logging requirements
   - Comprehensive event tracking
   - Tamper evidence (timestamps)
   - Data retention policies

6. MAINTENANCE
   - PruneOldRecords: Remove records older than N days
   - Vacuum: Optimize database size
   - Connection pooling for performance
   - Automatic schema migration on startup

7. PRODUCTION CONSIDERATIONS
   - Use external database (PostgreSQL) for HA
   - Implement replication for backup
   - Set up automated backups
   - Monitor database size and performance
   - Regular pruning of old records
   - Encryption at rest for sensitive data

*/
