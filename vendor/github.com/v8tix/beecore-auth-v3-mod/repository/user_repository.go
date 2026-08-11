package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/v8tix/beecore-auth-v3-mod/model"
)

var (
	ErrQuery        = errors.New("query error")
	ErrUserNotFound = errors.New("user not found")
	ErrRowScan      = errors.New("row scan error")
	ErrRowIteration = errors.New("row iteration error")
	ErrEmptyRoles   = errors.New("roles cannot be empty or nil")
)

func (r Repository) findUserByEmailAndPassword(ctx context.Context, email, password string) (*model.User, error) {
	var user model.User

	err := r.db.QueryRow(ctx, `
		SELECT 
		    id, 
		    dni, 
		    first_name, 
		    last_name, 
		    email, 
		    password, 
		    birthday, 
		    genre, 
		    phone, 
		    img_url, 
		    website, 
		    enabled, 
		    created_at, 
		    updated_at
		FROM users.users
		WHERE email = $1`, email).
		Scan(
			&user.ID,
			&user.DNI,
			&user.FirstName,
			&user.LastName,
			&user.Email,
			&user.Password.Hash,
			&user.Birthday,
			&user.Genre,
			&user.Phone,
			&user.ImgURL,
			&user.Website,
			&user.Enabled,
			&user.CreatedAt,
			&user.UpdatedAt,
		)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("findUserByEmailAndPassword: User not found", "error", err)
			return nil, fmt.Errorf("%w: %v", ErrUserNotFound, err)
		}
		slog.Error("findUserByEmailAndPassword: Query error", "error", err)
		return nil, fmt.Errorf("%w: %v", ErrQuery, err)
	}

	err = user.Password.Matches(password)
	if err != nil {
		slog.Error("findUserByEmailAndPassword: Password mismatch", "error", err)
		return nil, err
	}

	return &user, nil
}

// findUserByID looks up a user without a password check — used by the
// refresh-token grant, where the caller has already been authenticated once
// (at login) and the password is deliberately not retained.
func (r Repository) findUserByID(ctx context.Context, userID string) (*model.User, error) {
	var user model.User

	err := r.db.QueryRow(ctx, `
		SELECT
		    id,
		    dni,
		    first_name,
		    last_name,
		    email,
		    password,
		    birthday,
		    genre,
		    phone,
		    img_url,
		    website,
		    enabled,
		    created_at,
		    updated_at
		FROM users.users
		WHERE id = $1`, userID).
		Scan(
			&user.ID,
			&user.DNI,
			&user.FirstName,
			&user.LastName,
			&user.Email,
			&user.Password.Hash,
			&user.Birthday,
			&user.Genre,
			&user.Phone,
			&user.ImgURL,
			&user.Website,
			&user.Enabled,
			&user.CreatedAt,
			&user.UpdatedAt,
		)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Error("findUserByID: User not found", "error", err)
			return nil, fmt.Errorf("%w: %v", ErrUserNotFound, err)
		}
		slog.Error("findUserByID: Query error", "error", err)
		return nil, fmt.Errorf("%w: %v", ErrQuery, err)
	}

	return &user, nil
}

func (r Repository) findUserRoles(ctx context.Context, userID string) ([]model.Role, error) {
	query := `
		SELECT r.id, r.name
		FROM users.roles r
		JOIN users.user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQuery, err)
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var role model.Role
		if err = rows.Scan(&role.ID, &role.Name); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRowScan, err)
		}
		roles = append(roles, role)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%w, %v", ErrRowIteration, err)
	}

	return roles, nil
}

func (r Repository) FindPermissionsByRolesWithHierarchy(ctx context.Context, roleIDs []string) ([]model.Permission, error) {
	if len(roleIDs) == 0 {
		return nil, ErrEmptyRoles
	}

	query := `
		WITH RECURSIVE effective_roles AS (
			SELECT id FROM users.roles WHERE id = ANY($1)
			UNION
			SELECT rh.parent_role_id
			FROM users.role_hierarchy rh
			JOIN effective_roles er ON rh.child_role_id = er.id
		)
		SELECT DISTINCT p.id, p.name
		FROM users.permissions p
		JOIN users.role_permissions rp ON rp.permission_id = p.id
		JOIN effective_roles er ON er.id = rp.role_id
	`

	roleArray := &pgtype.TextArray{}
	if err := roleArray.Set(roleIDs); err != nil {
		return nil, fmt.Errorf("failed to convert role IDs to TEXT[]: %w", err)
	}

	rows, err := r.db.Query(ctx, query, roleArray)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQuery, err)
	}
	defer rows.Close()

	permSet := make(map[string]model.Permission)
	for rows.Next() {
		var permission model.Permission
		if err := rows.Scan(&permission.ID, &permission.Name); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRowScan, err)
		}
		permSet[permission.ID] = permission
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w, %v", ErrRowIteration, err)
	}

	permissions := make([]model.Permission, 0, len(permSet))
	for _, perm := range permSet {
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

func (r Repository) FindPermissionsByUserID(ctx context.Context, userID string) ([]model.Permission, error) {
	const query = `
		WITH RECURSIVE effective_roles AS (
			SELECT ur.role_id
			FROM users.user_roles ur
			WHERE ur.user_id = $1

			UNION

			SELECT rh.parent_role_id
			FROM users.role_hierarchy rh
			JOIN effective_roles er ON rh.child_role_id = er.role_id
		)
		SELECT DISTINCT p.id, p.name
		FROM effective_roles er
		JOIN users.role_permissions rp ON rp.role_id = er.role_id
		JOIN users.permissions p ON p.id = rp.permission_id
		ORDER BY p.name;
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQuery, err)
	}
	defer rows.Close()

	var permissions []model.Permission
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRowScan, err)
		}
		permissions = append(permissions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRowIteration, err)
	}

	return permissions, nil
}
