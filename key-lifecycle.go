// key-lifecycle.go - Key Lifecycle Management for FIPS 140-2 Compliance
package main

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// KeyLifecycleState defines key lifecycle states
type KeyLifecycleState int

const (
	StateGenerated KeyLifecycleState = iota
	StateActivated
	StateRotating
	StateDeactivated
	StateDestroyed
)

// KeyLifecycle tracks key lifecycle
type KeyLifecycle struct {
	KeyID          string
	KeyMaterial    [32]byte
	Generated      time.Time
	Activated      time.Time
	RotationDue    time.Time
	Deactivated    time.Time
	Destroyed      time.Time
	State          KeyLifecycleState
	RotationCount  int
	AccessCount    int64
	LastAccess     time.Time
	Zeroized       bool
	CreatedBy      string
	RotatedBy      string
	DestroyedBy    string
	AuditTrail     []AuditEntry
	mu             sync.RWMutex
}

// KeyLifecycleManager manages all key lifecycles
type KeyLifecycleManager struct {
	keys       map[string]*KeyLifecycle
	hsm        *HSMIntegration
	rotationInterval time.Duration
	mu         sync.RWMutex
}

// NewKeyLifecycleManager creates new lifecycle manager
func NewKeyLifecycleManager(hsm *HSMIntegration) *KeyLifecycleManager {
	return &KeyLifecycleManager{
		keys:             make(map[string]*KeyLifecycle),
		hsm:              hsm,
		rotationInterval: 365 * 24 * time.Hour, // Annual rotation
	}
}

// GenerateKey generates new key with tracking
func (klm *KeyLifecycleManager) GenerateKey(keyID string, operatorID string) (*KeyLifecycle, error) {
	klm.mu.Lock()
	defer klm.mu.Unlock()

	// Check if key already exists
	if _, exists := klm.keys[keyID]; exists {
		return nil, fmt.Errorf("key %s already exists", keyID)
	}

	// Generate key material
	keyMaterial := [32]byte{}
	if _, err := rand.Read(keyMaterial[:]); err != nil {
		return nil, err
	}

	now := time.Now()
	keyLifecycle := &KeyLifecycle{
		KeyID:       keyID,
		KeyMaterial: keyMaterial,
		Generated:   now,
		State:       StateGenerated,
		CreatedBy:   operatorID,
		AuditTrail:  make([]AuditEntry, 0),
	}

	// Import to HSM
	if klm.hsm != nil {
		if err := klm.hsm.ImportKey(keyMaterial); err != nil {
			return nil, fmt.Errorf("failed to import key to HSM: %v", err)
		}
	}

	// Add to tracking
	klm.keys[keyID] = keyLifecycle

	// Audit entry
	keyLifecycle.addAuditEntry("KEY_GENERATED", fmt.Sprintf("Key %s generated", keyID), "SUCCESS", operatorID)

	return keyLifecycle, nil
}

// ActivateKey activates a generated key
func (klm *KeyLifecycleManager) ActivateKey(keyID string, operatorID string) error {
	klm.mu.Lock()
	defer klm.mu.Unlock()

	keyLC, exists := klm.keys[keyID]
	if !exists {
		return fmt.Errorf("key %s not found", keyID)
	}

	keyLC.mu.Lock()
	defer keyLC.mu.Unlock()

	if keyLC.State != StateGenerated {
		return fmt.Errorf("key must be in Generated state to activate")
	}

	keyLC.Activated = time.Now()
	keyLC.RotationDue = keyLC.Activated.Add(keyLC.RotationDue.Sub(keyLC.Generated))
	keyLC.State = StateActivated

	keyLC.addAuditEntry("KEY_ACTIVATED", fmt.Sprintf("Key %s activated", keyID), "SUCCESS", operatorID)

	return nil
}

// RotateKey rotates key material
func (klm *KeyLifecycleManager) RotateKey(keyID string, operatorID string) (*KeyLifecycle, error) {
	klm.mu.Lock()
	defer klm.mu.Unlock()

	keyLC, exists := klm.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key %s not found", keyID)
	}

	keyLC.mu.Lock()
	defer keyLC.mu.Unlock()

	if keyLC.State != StateActivated {
		return nil, fmt.Errorf("only activated keys can be rotated")
	}

	// Generate new key material
	newKeyMaterial := [32]byte{}
	if _, err := rand.Read(newKeyMaterial[:]); err != nil {
		return nil, err
	}

	// Save old key for audit
	oldKeyMaterial := keyLC.KeyMaterial

	// Update with new material
	keyLC.KeyMaterial = newKeyMaterial
	keyLC.RotationCount++
	keyLC.RotatedBy = operatorID
	keyLC.RotationDue = time.Now().Add(keyLC.RotationDue.Sub(keyLC.RotationDue))

	// Import new key to HSM
	if klm.hsm != nil {
		if err := klm.hsm.ImportKey(newKeyMaterial); err != nil {
			// Restore old key on failure
			keyLC.KeyMaterial = oldKeyMaterial
			return nil, fmt.Errorf("failed to import rotated key to HSM: %v", err)
		}
	}

	keyLC.addAuditEntry("KEY_ROTATED", fmt.Sprintf("Key %s rotated (count: %d)", keyID, keyLC.RotationCount), "SUCCESS", operatorID)

	return keyLC, nil
}

// DeactivateKey deactivates a key
func (klm *KeyLifecycleManager) DeactivateKey(keyID string, operatorID string) error {
	klm.mu.Lock()
	defer klm.mu.Unlock()

	keyLC, exists := klm.keys[keyID]
	if !exists {
		return fmt.Errorf("key %s not found", keyID)
	}

	keyLC.mu.Lock()
	defer keyLC.mu.Unlock()

	keyLC.Deactivated = time.Now()
	keyLC.State = StateDeactivated
	keyLC.DestroyedBy = operatorID

	keyLC.addAuditEntry("KEY_DEACTIVATED", fmt.Sprintf("Key %s deactivated", keyID), "SUCCESS", operatorID)

	return nil
}

// ZeroizeKey securely wipes key material
func (klm *KeyLifecycleManager) ZeroizeKey(keyID string, operatorID string) error {
	klm.mu.Lock()
	defer klm.mu.Unlock()

	keyLC, exists := klm.keys[keyID]
	if !exists {
		return fmt.Errorf("key %s not found", keyID)
	}

	keyLC.mu.Lock()
	defer keyLC.mu.Unlock()

	// Overwrite key material with zeros
	for i := 0; i < 32; i++ {
		keyLC.KeyMaterial[i] = 0
	}

	keyLC.Destroyed = time.Now()
	keyLC.State = StateDestroyed
	keyLC.Zeroized = true

	keyLC.addAuditEntry("KEY_ZEROIZED", fmt.Sprintf("Key %s securely destroyed", keyID), "SUCCESS", operatorID)

	return nil
}

// GetKeyStatus returns key lifecycle status
func (klm *KeyLifecycleManager) GetKeyStatus(keyID string) (*KeyLifecycle, error) {
	klm.mu.RLock()
	defer klm.mu.RUnlock()

	keyLC, exists := klm.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key %s not found", keyID)
	}

	return keyLC, nil
}

// GetKeysNeedingRotation returns keys that need rotation
func (klm *KeyLifecycleManager) GetKeysNeedingRotation() []string {
	klm.mu.RLock()
	defer klm.mu.RUnlock()

	needsRotation := make([]string, 0)
	now := time.Now()

	for keyID, keyLC := range klm.keys {
		keyLC.mu.RLock()
		if keyLC.State == StateActivated && now.After(keyLC.RotationDue) {
			needsRotation = append(needsRotation, keyID)
		}
		keyLC.mu.RUnlock()
	}

	return needsRotation
}

// addAuditEntry adds entry to key's audit trail
func (kl *KeyLifecycle) addAuditEntry(eventType, description, status, operatorID string) {
	entry := AuditEntry{
		Timestamp:   time.Now(),
		EventType:   eventType,
		Description: description,
		Status:      status,
		OperatorID:  operatorID,
	}
	kl.AuditTrail = append(kl.AuditTrail, entry)
}

// GetAuditTrail returns key's audit trail
func (klm *KeyLifecycleManager) GetAuditTrail(keyID string) []AuditEntry {
	klm.mu.RLock()
	defer klm.mu.RUnlock()

	keyLC, exists := klm.keys[keyID]
	if !exists {
		return []AuditEntry{}
	}

	keyLC.mu.RLock()
	defer keyLC.mu.RUnlock()

	// Return copy of audit trail
	trailCopy := make([]AuditEntry, len(keyLC.AuditTrail))
	copy(trailCopy, keyLC.AuditTrail)
	return trailCopy
}

// PrintKeyLifecycleStatus prints lifecycle status
func (klm *KeyLifecycleManager) PrintKeyLifecycleStatus() {
	klm.mu.RLock()
	defer klm.mu.RUnlock()

	fmt.Printf("\n🔑 Key Lifecycle Management Status:\n")
	fmt.Printf("   Total Keys: %d\n", len(klm.keys))

	for keyID, keyLC := range klm.keys {
		keyLC.mu.RLock()
		stateStr := ""
		switch keyLC.State {
		case StateGenerated:
			stateStr = "Generated"
		case StateActivated:
			stateStr = "Activated"
		case StateRotating:
			stateStr = "Rotating"
		case StateDeactivated:
			stateStr = "Deactivated"
		case StateDestroyed:
			stateStr = "Destroyed"
		}

		fmt.Printf("   Key: %s\n", keyID)
		fmt.Printf("     State:        %s\n", stateStr)
		fmt.Printf("     Generated:    %v\n", keyLC.Generated)
		fmt.Printf("     Rotations:    %d\n", keyLC.RotationCount)
		fmt.Printf("     Zeroized:     %v\n", keyLC.Zeroized)
		fmt.Printf("     Audit Events: %d\n", len(keyLC.AuditTrail))
		keyLC.mu.RUnlock()
	}
}

// StateString returns string representation of state
func (s KeyLifecycleState) String() string {
	states := []string{"Generated", "Activated", "Rotating", "Deactivated", "Destroyed"}
	if s >= 0 && s < KeyLifecycleState(len(states)) {
		return states[s]
	}
	return "Unknown"
}
