package auth

import (
	"context"
	"database/sql"
	"fmt"
)

// UserRecord binds an OIDC subject (or static username) to a role + tenant.
// Story 21.1 + 21.4: OIDC integration is represented by the subject claim
// that callers supply — the actual token exchange is out of scope for v1.0.
type UserRecord struct {
	Subject  string
	Role     Role
	TenantID string
}

// UserStore persists the RBAC user directory.
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a user store.
func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }

// Upsert inserts or updates a user record.
func (s *UserStore) Upsert(ctx context.Context, u UserRecord) error {
	if !IsValidRole(string(u.Role)) {
		return fmt.Errorf("invalid role: %s", u.Role)
	}
	if u.TenantID == "" {
		u.TenantID = "default"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO rbac_users (subject, role, tenant_id) VALUES (?, ?, ?)
		 ON CONFLICT(subject) DO UPDATE SET role = excluded.role, tenant_id = excluded.tenant_id`,
		u.Subject, string(u.Role), u.TenantID)
	return err
}

// Delete removes a user by subject.
func (s *UserStore) Delete(ctx context.Context, subject string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM rbac_users WHERE subject = ?`, subject)
	return err
}

// Get returns the user record for a subject.
func (s *UserStore) Get(ctx context.Context, subject string) (*UserRecord, error) {
	var u UserRecord
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT subject, role, tenant_id FROM rbac_users WHERE subject = ?`, subject,
	).Scan(&u.Subject, &role, &u.TenantID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user %s not found", subject)
	}
	if err != nil {
		return nil, err
	}
	u.Role = Role(role)
	return &u, nil
}

// List returns all user records.
func (s *UserStore) List(ctx context.Context) ([]UserRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT subject, role, tenant_id FROM rbac_users ORDER BY subject`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var u UserRecord
		var role string
		if err := rows.Scan(&u.Subject, &role, &u.TenantID); err != nil {
			return nil, err
		}
		u.Role = Role(role)
		users = append(users, u)
	}
	return users, rows.Err()
}

// IsValidRole reports whether a string is a known role.
func IsValidRole(r string) bool {
	switch Role(r) {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	}
	return false
}

// TenantContextKey is the typed context key middleware use to store and
// retrieve the resolved tenant ID on a request.
type TenantContextKey struct{}
