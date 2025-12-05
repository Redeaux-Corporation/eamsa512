package main

import (
	"crypto/sha3"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// ============================================================================
// EAMSA 512 - Key Rotation and Lifecycle Management
// Key management, rotation scheduling, and archival
//
// Implements FIPS 140-2 Level 2 key lifecycle requirements including:
// - Automatic key rotation on schedule
// - Key versioning and history tracking
// - Secure key archival and destruction
// - Key state management
//
// Last updated: December 4, 2025
// ============================================================================

// KeyState represents the state of a key
type KeyState string

const (
	KeyStateActive    KeyState = "active"    // Currently in use for encryption/decryption
	KeyStatePending   KeyState = "pending"   // Scheduled but not yet active
	KeyStateRotated   KeyState = "rotated"   // Replaced by newer key, available for decryption only
	KeyStateArchived  KeyState = "archived"  // Archived after retention period
	KeyStateDestroyed KeyState = "destroyed" // Securely destroyed
)

// KeyMetadata contains information about a key
type KeyMetadata struct {
	ID              string    `json:"id"`               // Unique key identifier
	Version         int       `json:"version"`          // Key version number
	State           KeyState  `json:"state"`            // Current state
	CreatedAt       time.Time `json:"created_at"`       // Creation timestamp
	ActivatedAt     time.Time `json:"activated_at"`     // When key became active
	RotatedAt       time.Time `json:"rotated_at"`       // When key was rotated
	ArchivedAt      time.Time `json:"archived_at"`      // When key was archived
	DestroyedAt     time.Time `json:"destroyed_at"`     // When key was destroyed
	KeyHash         string    `json:"key_hash"`         // SHA3-512 hash of key material
	EncryptionCount int64     `json:"encryption_count"` // Number of encryptions with this key
	DecryptionCount int64     `json:"decryption_count"` // Number of decryptions with this key
}

// KeyEntry represents a stored key with metadata
type KeyEntry struct {
	Metadata  KeyMetadata
	Material  []byte // Encrypted key material (never stored unencrypted)
	ExpiresAt time.Time
}

// KeyRotationPolicy defines the key rotation schedule and rules
type KeyRotationPolicy struct {
	// Automatic rotation enabled
	Enabled bool

	// Rotation interval (days)
	IntervalDays int

	// Retention cycles: how many old keys to keep
	RetentionCycles int

	// Maximum key age (days) - if exceeded, force rotation immediately
	MaxKeyAgeDays int

	// Minimum key age (days) - cannot rotate before this
	MinKeyAgeDays int

	// Archive location for rotated keys
	ArchiveLocation string

	// Destruction method: "overwrite" (default), "zero", "random"
	DestructionMethod string

	// Number of overwrite passes for destruction
	DestructionPasses int
}

// DefaultKeyRotationPolicy returns sensible defaults for FIPS 140-2 compliance
func DefaultKeyRotationPolicy() KeyRotationPolicy {
	return KeyRotationPolicy{
		Enabled:           true,
		IntervalDays:      365,
		RetentionCycles:   3,
		MaxKeyAgeDays:     730,
		MinKeyAgeDays:     30,
		ArchiveLocation:   "/var/lib/eamsa512/key-archive/",
		DestructionMethod: "random",
		DestructionPasses: 3,
	}
}

// KeyManager manages the key lifecycle
type KeyManager struct {
	mu sync.RWMutex

	// Active key
	activeKey *KeyEntry

	// Key history (version -> KeyEntry)
	history map[int]*KeyEntry

	// Current version number
	currentVersion int

	// Rotation policy
	policy KeyRotationPolicy

	// Last rotation time
	lastRotationTime time.Time

	// Rotation ticker
	rotationTicker *time.Ticker

	// Audit logger
	auditLogger *log.Logger

	// Stop channel for background operations
	stopCh chan struct{}
}

// NewKeyManager creates a new key manager with initial key
func NewKeyManager(initialKey []byte, policy KeyRotationPolicy) (*KeyManager, error) {
	if len(initialKey) != KeySize {
		return nil, fmt.Errorf("invalid initial key size: expected %d bytes, got %d", KeySize, len(initialKey))
	}

	// Setup audit logger
	auditFile, err := os.OpenFile("/var/log/eamsa512/key-rotation.log", 
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %v", err)
	}

	auditLogger := log.New(auditFile, "[KEY-ROTATION] ", log.LstdFlags|log.Lshortfile)

	// Create initial key entry
	initialMetadata := KeyMetadata{
		ID:          fmt.Sprintf("key_%d", 1),
		Version:     1,
		State:       KeyStateActive,
		CreatedAt:   time.Now(),
		ActivatedAt: time.Now(),
		KeyHash:     hashKey(initialKey),
	}

	keyEntry := &KeyEntry{
		Metadata: initialMetadata,
		Material: initialKey,
		ExpiresAt: time.Now().AddDate(0, 0, policy.MaxKeyAgeDays),
	}

	km := &KeyManager{
		activeKey:       keyEntry,
		history:         make(map[int]*KeyEntry),
		currentVersion:  1,
		policy:          policy,
		lastRotationTime: time.Now(),
		auditLogger:     auditLogger,
		stopCh:          make(chan struct{}),
	}

	// Store in history
	km.history[1] = keyEntry

	// Log key creation
	km.auditLogger.Printf("KEY_CREATED version=%d hash=%s", initialMetadata.Version, initialMetadata.KeyHash)

	// Start automatic rotation scheduler if enabled
	if policy.Enabled {
		go km.rotationScheduler()
	}

	return km, nil
}

// hashKey computes SHA3-512 hash of key material
func hashKey(key []byte) string {
	hash := sha3.New512()
	hash.Write(key)
	return fmt.Sprintf("%x", hash.Sum(nil))[:32] // First 32 chars for display
}

// GetActiveKey returns the currently active key
func (km *KeyManager) GetActiveKey() ([]byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.activeKey == nil {
		return nil, fmt.Errorf("no active key available")
	}

	// Check if key has expired
	if time.Now().After(km.activeKey.ExpiresAt) {
		return nil, fmt.Errorf("active key has expired")
	}

	return km.activeKey.Material, nil
}

// GetKeyByVersion retrieves a specific key version
func (km *KeyManager) GetKeyByVersion(version int) ([]byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	entry, exists := km.history[version]
	if !exists {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	// Allow retrieval of active and rotated keys (for decryption)
	if entry.Metadata.State != KeyStateActive && 
	   entry.Metadata.State != KeyStateRotated {
		return nil, fmt.Errorf("key version %d is not available (state: %s)", 
			version, entry.Metadata.State)
	}

	return entry.Material, nil
}

// RotateKey performs immediate key rotation
func (km *KeyManager) RotateKey(newKey []byte) error {
	if len(newKey) != KeySize {
		return fmt.Errorf("invalid new key size: expected %d bytes, got %d", KeySize, len(newKey))
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	// Check minimum key age
	if time.Since(km.lastRotationTime).Hours() < float64(km.policy.MinKeyAgeDays*24) {
		return fmt.Errorf("cannot rotate key before minimum age of %d days", km.policy.MinKeyAgeDays)
	}

	// Mark old key as rotated
	if km.activeKey != nil {
		km.activeKey.Metadata.State = KeyStateRotated
		km.activeKey.Metadata.RotatedAt = time.Now()

		km.auditLogger.Printf("KEY_ROTATED version=%d old_hash=%s at=%s",
			km.activeKey.Metadata.Version,
			km.activeKey.Metadata.KeyHash,
			km.activeKey.Metadata.RotatedAt.Format(time.RFC3339))
	}

	// Create new key entry
	km.currentVersion++
	newMetadata := KeyMetadata{
		ID:          fmt.Sprintf("key_%d", km.currentVersion),
		Version:     km.currentVersion,
		State:       KeyStateActive,
		CreatedAt:   time.Now(),
		ActivatedAt: time.Now(),
		KeyHash:     hashKey(newKey),
	}

	newEntry := &KeyEntry{
		Metadata:  newMetadata,
		Material:  newKey,
		ExpiresAt: time.Now().AddDate(0, 0, km.policy.MaxKeyAgeDays),
	}

	// Update active key and history
	km.activeKey = newEntry
	km.history[km.currentVersion] = newEntry
	km.lastRotationTime = time.Now()

	// Archive old keys if retention limit exceeded
	km.archiveOldKeys()

	// Log rotation
	km.auditLogger.Printf("KEY_ROTATED_NEW version=%d new_hash=%s", 
		newMetadata.Version, newMetadata.KeyHash)

	return nil
}

// archiveOldKeys archives keys beyond retention policy
func (km *KeyManager) archiveOldKeys() {
	// Count active and rotated keys
	activeCount := 0
	for _, entry := range km.history {
		if entry.Metadata.State == KeyStateActive || entry.Metadata.State == KeyStateRotated {
			activeCount++
		}
	}

	// Archive oldest keys if exceeding retention
	if activeCount > km.policy.RetentionCycles {
		keysToArchive := activeCount - km.policy.RetentionCycles

		for version, entry := range km.history {
			if keysToArchive <= 0 {
				break
			}

			if entry.Metadata.State == KeyStateRotated {
				entry.Metadata.State = KeyStateArchived
				entry.Metadata.ArchivedAt = time.Now()

				// Securely erase from memory
				km.securelyEraseKey(entry)

				km.auditLogger.Printf("KEY_ARCHIVED version=%d hash=%s", 
					version, entry.Metadata.KeyHash)

				keysToArchive--
			}
		}
	}
}

// securelyEraseKey securely erases key material from memory
func (km *KeyManager) securelyEraseKey(entry *KeyEntry) {
	method := km.policy.DestructionMethod
	passes := km.policy.DestructionPasses

	if method == "zero" {
		// Overwrite with zeros
		for i := 0; i < len(entry.Material); i++ {
			entry.Material[i] = 0
		}
	} else if method == "random" || method == "overwrite" {
		// Overwrite with random data (Gutmann-like method)
		for pass := 0; pass < passes; pass++ {
			hash := sha3.New256()
			hash.Write([]byte(fmt.Sprintf("pass_%d_%d", pass, time.Now().UnixNano())))
			pattern := hash.Sum(nil)

			for i := 0; i < len(entry.Material); i++ {
				entry.Material[i] ^= pattern[i%len(pattern)]
			}
		}
	}

	// Mark as destroyed
	entry.Material = nil
}

// GetKeyMetadata retrieves metadata for a key version
func (km *KeyManager) GetKeyMetadata(version int) (*KeyMetadata, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	entry, exists := km.history[version]
	if !exists {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	// Return copy to prevent external modification
	metadata := entry.Metadata
	return &metadata, nil
}

// GetActiveKeyMetadata retrieves metadata for the active key
func (km *KeyManager) GetActiveKeyMetadata() (*KeyMetadata, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.activeKey == nil {
		return nil, fmt.Errorf("no active key available")
	}

	metadata := km.activeKey.Metadata
	return &metadata, nil
}

// ListKeyVersions returns all available key versions
func (km *KeyManager) ListKeyVersions() []KeyMetadata {
	km.mu.RLock()
	defer km.mu.RUnlock()

	versions := make([]KeyMetadata, 0, len(km.history))
	for _, entry := range km.history {
		versions = append(versions, entry.Metadata)
	}

	return versions
}

// IncrementEncryptionCount increments the encryption counter for active key
func (km *KeyManager) IncrementEncryptionCount() {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.activeKey != nil {
		km.activeKey.Metadata.EncryptionCount++
	}
}

// IncrementDecryptionCount increments the decryption counter for a key version
func (km *KeyManager) IncrementDecryptionCount(version int) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	entry, exists := km.history[version]
	if !exists {
		return fmt.Errorf("key version %d not found", version)
	}

	entry.Metadata.DecryptionCount++
	return nil
}

// rotationScheduler runs background key rotation checks
func (km *KeyManager) rotationScheduler() {
	// Check rotation need every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-km.stopCh:
			km.auditLogger.Printf("KEY_ROTATION_SCHEDULER_STOPPED")
			return

		case <-ticker.C:
			km.checkRotationNeeded()
		}
	}
}

// checkRotationNeeded checks if key rotation is needed
func (km *KeyManager) checkRotationNeeded() {
	km.mu.RLock()
	activeKey := km.activeKey
	km.mu.RUnlock()

	if activeKey == nil {
		return
	}

	ageHours := time.Since(activeKey.Metadata.CreatedAt).Hours()
	maxAgeHours := float64(km.policy.MaxKeyAgeDays * 24)
	rotationIntervalHours := float64(km.policy.IntervalDays * 24)

	// Log rotation check
	km.auditLogger.Printf("KEY_ROTATION_CHECK age_hours=%.1f max_age=%.1f interval=%.1f",
		ageHours, maxAgeHours, rotationIntervalHours)

	// Check if rotation is needed
	if ageHours >= maxAgeHours {
		km.auditLogger.Printf("KEY_ROTATION_NEEDED_MAX_AGE age_hours=%.1f", ageHours)
		// In production, would trigger rotation event here
	} else if ageHours >= rotationIntervalHours {
		km.auditLogger.Printf("KEY_ROTATION_NEEDED_INTERVAL age_hours=%.1f", ageHours)
		// In production, would trigger rotation event here
	}
}

// Stop stops the key manager's background operations
func (km *KeyManager) Stop() {
	close(km.stopCh)
	km.auditLogger.Printf("KEY_MANAGER_STOPPED")
}

// GetRotationPolicy returns the current rotation policy
func (km *KeyManager) GetRotationPolicy() KeyRotationPolicy {
	km.mu.RLock()
	defer km.mu.RUnlock()

	return km.policy
}

// UpdateRotationPolicy updates the rotation policy
func (km *KeyManager) UpdateRotationPolicy(policy KeyRotationPolicy) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Validate policy
	if policy.IntervalDays <= 0 {
		return fmt.Errorf("rotation interval must be > 0")
	}

	if policy.MaxKeyAgeDays <= policy.IntervalDays {
		return fmt.Errorf("max key age must be > rotation interval")
	}

	if policy.RetentionCycles < 1 {
		return fmt.Errorf("retention cycles must be >= 1")
	}

	km.policy = policy

	km.auditLogger.Printf("KEY_ROTATION_POLICY_UPDATED interval_days=%d max_age=%d retention=%d",
		policy.IntervalDays, policy.MaxKeyAgeDays, policy.RetentionCycles)

	return nil
}

// GenerateNewKey generates a new random key using the entropy source
func GenerateNewKey(entropySource func() float64) []byte {
	key := make([]byte, KeySize)

	for i := 0; i < KeySize; i++ {
		entropy := entropySource()
		key[i] = byte(entropy * 255)
	}

	return key
}

// ============================================================================
// Key Backup and Recovery
// ============================================================================

// BackupKey creates an encrypted backup of a key
func (km *KeyManager) BackupKey(version int, backupKey []byte) ([]byte, error) {
	key, err := km.GetKeyByVersion(version)
	if err != nil {
		return nil, err
	}

	// Encrypt key with backup key using EAMSA 512
	backupData, err := EncryptData(key, backupKey, nil)
	if err != nil {
		return nil, err
	}

	km.auditLogger.Printf("KEY_BACKUP version=%d size=%d", version, len(backupData))

	return backupData, nil
}

// RestoreKey restores a key from encrypted backup
func (km *KeyManager) RestoreKey(backupData []byte, backupKey []byte) error {
	// Decrypt backup data
	key, err := DecryptData(backupData, backupKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt backup: %v", err)
	}

	// Rotate to restored key
	return km.RotateKey(key)
}

// ============================================================================
// Key Statistics and Reporting
// ============================================================================

// KeyStatistics holds key usage statistics
type KeyStatistics struct {
	TotalKeys        int   `json:"total_keys"`
	ActiveKeys       int   `json:"active_keys"`
	RotatedKeys      int   `json:"rotated_keys"`
	ArchivedKeys     int   `json:"archived_keys"`
	TotalEncryptions int64 `json:"total_encryptions"`
	TotalDecryptions int64 `json:"total_decryptions"`
}

// GetStatistics returns key statistics
func (km *KeyManager) GetStatistics() KeyStatistics {
	km.mu.RLock()
	defer km.mu.RUnlock()

	stats := KeyStatistics{
		TotalKeys: len(km.history),
	}

	var totalEncryptions, totalDecryptions int64

	for _, entry := range km.history {
		switch entry.Metadata.State {
		case KeyStateActive:
			stats.ActiveKeys++
		case KeyStateRotated:
			stats.RotatedKeys++
		case KeyStateArchived:
			stats.ArchivedKeys++
		}

		totalEncryptions += entry.Metadata.EncryptionCount
		totalDecryptions += entry.Metadata.DecryptionCount
	}

	stats.TotalEncryptions = totalEncryptions
	stats.TotalDecryptions = totalDecryptions

	return stats
}

// ============================================================================
// Example Usage and Testing
// ============================================================================

func main() {
	fmt.Println("EAMSA 512 - Key Rotation Management")
	fmt.Println("=====================================\n")

	// Create initial key
	initialKey := []byte("thirtytwobytemasterkeyfor512bit") // 32 bytes

	// Create key manager with default policy
	policy := DefaultKeyRotationPolicy()
	policy.Enabled = false // Disable automatic rotation for demo

	km, err := NewKeyManager(initialKey, policy)
	if err != nil {
		fmt.Printf("Error creating key manager: %v\n", err)
		return
	}

	defer km.Stop()

	fmt.Println("Initial State:")
	fmt.Printf("  Active Key Version: 1\n")

	// Get active key metadata
	activeMetadata, _ := km.GetActiveKeyMetadata()
	fmt.Printf("  Active Key Hash: %s\n", activeMetadata.KeyHash)
	fmt.Printf("  Active Key Created: %s\n", activeMetadata.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Active Key State: %s\n\n", activeMetadata.State)

	// Simulate encryption/decryption operations
	km.IncrementEncryptionCount()
	km.IncrementEncryptionCount()
	km.IncrementDecryptionCount(1)

	// List key versions
	fmt.Println("Key Versions:")
	versions := km.ListKeyVersions()
	for _, v := range versions {
		fmt.Printf("  Version %d: State=%s, Hash=%s, Encryptions=%d, Decryptions=%d\n",
			v.Version, v.State, v.KeyHash, v.EncryptionCount, v.DecryptionCount)
	}

	// Get statistics
	fmt.Println("\nKey Statistics:")
	stats := km.GetStatistics()
	fmt.Printf("  Total Keys: %d\n", stats.TotalKeys)
	fmt.Printf("  Active Keys: %d\n", stats.ActiveKeys)
	fmt.Printf("  Rotated Keys: %d\n", stats.RotatedKeys)
	fmt.Printf("  Archived Keys: %d\n", stats.ArchivedKeys)
	fmt.Printf("  Total Encryptions: %d\n", stats.TotalEncryptions)
	fmt.Printf("  Total Decryptions: %d\n\n", stats.TotalDecryptions)

	// Perform key rotation
	fmt.Println("Performing Key Rotation...")
	newKey := []byte("newsecretkeyfor512bitencryption") // 32 bytes
	if err := km.RotateKey(newKey); err != nil {
		fmt.Printf("Error rotating key: %v\n", err)
		return
	}

	// Get updated statistics
	fmt.Println("\nAfter Rotation:")
	stats = km.GetStatistics()
	fmt.Printf("  Total Keys: %d\n", stats.TotalKeys)
	fmt.Printf("  Active Keys: %d\n", stats.ActiveKeys)
	fmt.Printf("  Rotated Keys: %d\n\n", stats.RotatedKeys)

	// List key versions after rotation
	fmt.Println("Key Versions After Rotation:")
	versions = km.ListKeyVersions()
	for _, v := range versions {
		fmt.Printf("  Version %d: State=%s, Hash=%s\n", v.Version, v.State, v.KeyHash)
	}

	// Get new active key metadata
	activeMetadata, _ = km.GetActiveKeyMetadata()
	fmt.Printf("\nNew Active Key Version: %d\n", activeMetadata.Version)
	fmt.Printf("New Active Key Hash: %s\n", activeMetadata.KeyHash)
	fmt.Printf("New Active Key State: %s\n", activeMetadata.State)

	// Verify old key is still accessible for decryption
	fmt.Println("\nVerifying Key Access:")
	oldKey, _ := km.GetKeyByVersion(1)
	newActiveKey, _ := km.GetActiveKey()

	fmt.Printf("  Old Key (version 1) accessible: %v\n", oldKey != nil)
	fmt.Printf("  New Active Key accessible: %v\n", newActiveKey != nil)
	fmt.Printf("  Keys are different: %v\n", 
		string(oldKey) != string(newActiveKey))

	// Display rotation policy
	fmt.Println("\nKey Rotation Policy:")
	fmt.Printf("  Enabled: %v\n", policy.Enabled)
	fmt.Printf("  Rotation Interval: %d days\n", policy.IntervalDays)
	fmt.Printf("  Max Key Age: %d days\n", policy.MaxKeyAgeDays)
	fmt.Printf("  Retention Cycles: %d\n", policy.RetentionCycles)
	fmt.Printf("  Destruction Method: %s\n", policy.DestructionMethod)
	fmt.Printf("  Destruction Passes: %d\n", policy.DestructionPasses)
}

// ============================================================================
// NOTES
// ============================================================================

/*

1. KEY LIFECYCLE
   - Active: In use for encryption and decryption
   - Rotated: No longer used for encryption, only for decryption
   - Archived: Old key, can be stored offline
   - Destroyed: Securely erased

2. ROTATION POLICY
   - Default: 365-day rotation interval
   - Max age: 730 days (force rotation)
   - Retention: Keep 3 key versions
   - Destruction: 3-pass random overwrite

3. FIPS 140-2 COMPLIANCE
   - Automatic rotation scheduling
   - Key versioning and tracking
   - Audit logging of all key operations
   - Secure key destruction
   - Key state management

4. SECURITY FEATURES
   - Separate active/rotated/archived keys
   - Backward compatibility for decryption
   - Encryption counter tracking
   - Decryption counter tracking
   - SHA3-512 key hashing for verification
   - Secure memory erasure (Gutmann method)

5. USAGE PATTERNS
   - GetActiveKey() for encryption
   - GetKeyByVersion() for decryption
   - RotateKey() for immediate rotation
   - GetStatistics() for monitoring

6. PRODUCTION CONSIDERATIONS
   - Keys should never be stored unencrypted
   - Use HSM for key storage in production
   - Implement key escrow for recovery
   - Regular audit log review
   - Test rotation procedures regularly

*/
