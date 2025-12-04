// rbac.go - Role-Based Access Control for FIPS 140-2 Compliance
package main

import (
	"fmt"
	"sync"
	"time"
)

// Role defines user roles in the system
type Role string

const (
	RoleAdmin       Role = "admin"       // Full system access
	RoleOperator    Role = "operator"    // Encrypt/decrypt operations
	RoleAuditor     Role = "auditor"     // Read-only access
	RoleMaintenance Role = "maintenance" // Key rotation and maintenance
)

// Permission defines what operations are allowed
type Permission string

const (
	PermEncrypt        Permission = "encrypt"
	PermDecrypt        Permission = "decrypt"
	PermGenerateKey    Permission = "generate_key"
	PermRotateKey      Permission = "rotate_key"
	PermDestroyKey     Permission = "destroy_key"
	PermViewAuditLog   Permission = "view_audit_log"
	PermModifyConfig   Permission = "modify_config"
	PermManageUsers    Permission = "manage_users"
)

// User represents a system user with RBAC
type User struct {
	UserID      string
	Username    string
	Role        Role
	CreatedAt   time.Time
	LastAccess  time.Time
	AccessCount int64
	Permissions []Permission
}

// RBACManager manages role-based access control
type RBACManager struct {
	users       map[string]*User
	rolePerms   map[Role][]Permission
	auditLog    []RBACEvent
	mu          sync.RWMutex
}

// RBACEvent logs access control events
type RBACEvent struct {
	Timestamp   time.Time
	UserID      string
	Username    string
	Action      string
	Resource    string
	Result      string
	Permission  Permission
	Details     string
}

// NewRBACManager creates new RBAC manager
func NewRBACManager() *RBACManager {
	rbac := &RBACManager{
		users:     make(map[string]*User),
		auditLog:  make([]RBACEvent, 0),
		rolePerms: make(map[Role][]Permission),
	}
	
	rbac.initializeRolePermissions()
	return rbac
}

// initializeRolePermissions sets up role-permission mappings
func (rbac *RBACManager) initializeRolePermissions() {
	// Admin: Full access
	rbac.rolePerms[RoleAdmin] = []Permission{
		PermEncrypt, PermDecrypt, PermGenerateKey, PermRotateKey,
		PermDestroyKey, PermViewAuditLog, PermModifyConfig, PermManageUsers,
	}
	
	// Operator: Encryption/decryption operations
	rbac.rolePerms[RoleOperator] = []Permission{
		PermEncrypt, PermDecrypt,
	}
	
	// Auditor: Read-only access
	rbac.rolePerms[RoleAuditor] = []Permission{
		PermViewAuditLog,
	}
	
	// Maintenance: Key management
	rbac.rolePerms[RoleMaintenance] = []Permission{
		PermGenerateKey, PermRotateKey, PermDestroyKey,
	}
}

// CreateUser creates new user with specified role
func (rbac *RBACManager) CreateUser(userID, username string, role Role) (*User, error) {
	rbac.mu.Lock()
	defer rbac.mu.Unlock()
	
	if _, exists := rbac.users[userID]; exists {
		return nil, fmt.Errorf("user %s already exists", userID)
	}
	
	perms, ok := rbac.rolePerms[role]
	if !ok {
		return nil, fmt.Errorf("invalid role: %s", role)
	}
	
	user := &User{
		UserID:      userID,
		Username:    username,
		Role:        role,
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		Permissions: perms,
	}
	
	rbac.users[userID] = user
	rbac.logEvent(RBACEvent{
		Timestamp:  time.Now(),
		UserID:     "system",
		Username:   "system",
		Action:     "CREATE_USER",
		Resource:   userID,
		Result:     "SUCCESS",
		Details:    fmt.Sprintf("Created user %s with role %s", username, role),
	})
	
	return user, nil
}

// CheckPermission verifies if user has permission for action
func (rbac *RBACManager) CheckPermission(userID string, permission Permission) bool {
	rbac.mu.RLock()
	defer rbac.mu.RUnlock()
	
	user, exists := rbac.users[userID]
	if !exists {
		rbac.logEvent(RBACEvent{
			Timestamp:  time.Now(),
			UserID:     userID,
			Action:     "PERMISSION_CHECK",
			Resource:   string(permission),
			Result:     "DENIED",
			Permission: permission,
			Details:    "User not found",
		})
		return false
	}
	
	// Check if user has permission
	for _, perm := range user.Permissions {
		if perm == permission {
			user.LastAccess = time.Now()
			user.AccessCount++
			return true
		}
	}
	
	rbac.logEvent(RBACEvent{
		Timestamp:  time.Now(),
		UserID:     userID,
		Username:   user.Username,
		Action:     "PERMISSION_CHECK",
		Resource:   string(permission),
		Result:     "DENIED",
		Permission: permission,
		Details:    fmt.Sprintf("User lacks permission: %s", permission),
	})
	
	return false
}

// AuthorizeAction verifies user can perform action and logs it
func (rbac *RBACManager) AuthorizeAction(userID string, action string, permission Permission) error {
	rbac.mu.RLock()
	user, exists := rbac.users[userID]
	rbac.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("user %s not found", userID)
	}
	
	// Check permission
	if !rbac.CheckPermission(userID, permission) {
		rbac.logEvent(RBACEvent{
			Timestamp:  time.Now(),
			UserID:     userID,
			Username:   user.Username,
			Action:     action,
			Result:     "DENIED",
			Permission: permission,
			Details:    "Access denied - insufficient permissions",
		})
		return fmt.Errorf("access denied: user %s cannot perform %s", userID, action)
	}
	
	// Log successful authorization
	rbac.logEvent(RBACEvent{
		Timestamp:  time.Now(),
		UserID:     userID,
		Username:   user.Username,
		Action:     action,
		Result:     "AUTHORIZED",
		Permission: permission,
		Details:    fmt.Sprintf("User authorized for: %s", action),
	})
	
	return nil
}

// GetUser retrieves user information
func (rbac *RBACManager) GetUser(userID string) (*User, error) {
	rbac.mu.RLock()
	defer rbac.mu.RUnlock()
	
	user, exists := rbac.users[userID]
	if !exists {
		return nil, fmt.Errorf("user %s not found", userID)
	}
	
	return user, nil
}

// UpdateUserRole changes user's role
func (rbac *RBACManager) UpdateUserRole(userID string, newRole Role) error {
	rbac.mu.Lock()
	defer rbac.mu.Unlock()
	
	user, exists := rbac.users[userID]
	if !exists {
		return fmt.Errorf("user %s not found", userID)
	}
	
	oldRole := user.Role
	perms, ok := rbac.rolePerms[newRole]
	if !ok {
		return fmt.Errorf("invalid role: %s", newRole)
	}
	
	user.Role = newRole
	user.Permissions = perms
	
	rbac.logEvent(RBACEvent{
		Timestamp:  time.Now(),
		UserID:     "system",
		Username:   "system",
		Action:     "ROLE_CHANGE",
		Resource:   userID,
		Result:     "SUCCESS",
		Details:    fmt.Sprintf("Changed role from %s to %s", oldRole, newRole),
	})
	
	return nil
}

// logEvent logs RBAC event
func (rbac *RBACManager) logEvent(event RBACEvent) {
	rbac.mu.Lock()
	defer rbac.mu.Unlock()
	
	rbac.auditLog = append(rbac.auditLog, event)
}

// GetAuditLog returns audit log entries
func (rbac *RBACManager) GetAuditLog() []RBACEvent {
	rbac.mu.RLock()
	defer rbac.mu.RUnlock()
	
	logCopy := make([]RBACEvent, len(rbac.auditLog))
	copy(logCopy, rbac.auditLog)
	return logCopy
}

// PrintRBACStatus prints current RBAC status
func (rbac *RBACManager) PrintRBACStatus() {
	rbac.mu.RLock()
	defer rbac.mu.RUnlock()
	
	fmt.Printf("\n👥 Role-Based Access Control (RBAC) Status:\n")
	fmt.Printf("   Total Users: %d\n", len(rbac.users))
	
	for _, user := range rbac.users {
		fmt.Printf("\n   User: %s (%s)\n", user.Username, user.UserID)
		fmt.Printf("     Role: %s\n", user.Role)
		fmt.Printf("     Permissions: %d\n", len(user.Permissions))
		fmt.Printf("     Created: %v\n", user.CreatedAt)
		fmt.Printf("     Last Access: %v\n", user.LastAccess)
		fmt.Printf("     Access Count: %d\n", user.AccessCount)
	}
	
	fmt.Printf("\n   Audit Log Events: %d\n", len(rbac.auditLog))
}

// VerifyRBACCompliance checks RBAC compliance
func (rbac *RBACManager) VerifyRBACCompliance() bool {
	rbac.mu.RLock()
	defer rbac.mu.RUnlock()
	
	// Check that all users have valid roles
	for _, user := range rbac.users {
		if _, ok := rbac.rolePerms[user.Role]; !ok {
			return false
		}
	}
	
	// Check that audit log exists
	if len(rbac.auditLog) == 0 {
		return false
	}
	
	return true
}
