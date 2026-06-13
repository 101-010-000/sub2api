package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type touchPieDeviceRepository struct {
	db *sql.DB
}

func NewTouchPieDeviceRepository(db *sql.DB) service.TouchPieDeviceRepository {
	return &touchPieDeviceRepository{db: db}
}

func (r *touchPieDeviceRepository) CreateDeviceSession(ctx context.Context, session *service.TouchPieDeviceSession) error {
	if r == nil || r.db == nil || session == nil {
		return service.ErrTouchPieDeviceNotFound
	}
	now := time.Now().UTC()
	session.CreatedAt = now
	session.UpdatedAt = now
	return r.db.QueryRowContext(ctx, `
		INSERT INTO touch_pie_device_sessions (
			device_code_hash, user_code_hash, status, expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING id
	`, session.DeviceCodeHash, session.UserCodeHash, session.Status, session.ExpiresAt, now).Scan(&session.ID)
}

func (r *touchPieDeviceRepository) GetDeviceSessionByUserCodeHash(ctx context.Context, hash string) (*service.TouchPieDeviceSession, error) {
	return r.getDeviceSession(ctx, "user_code_hash", hash)
}

func (r *touchPieDeviceRepository) GetDeviceSessionByDeviceCodeHash(ctx context.Context, hash string) (*service.TouchPieDeviceSession, error) {
	return r.getDeviceSession(ctx, "device_code_hash", hash)
}

func (r *touchPieDeviceRepository) ApproveDeviceSession(ctx context.Context, id int64, userID int64, now time.Time) error {
	if r == nil || r.db == nil {
		return service.ErrTouchPieDeviceNotFound
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE touch_pie_device_sessions
		SET status = $1, user_id = $2, approved_at = $3, updated_at = $3
		WHERE id = $4 AND status IN ($5, $1) AND consumed_at IS NULL AND expires_at > $3
	`, service.TouchPieDeviceStatusApproved, userID, now, id, service.TouchPieDeviceStatusPending)
	return touchPieRowsAffected(res, err)
}

func (r *touchPieDeviceRepository) ConsumeDeviceSession(ctx context.Context, id int64, now time.Time) error {
	if r == nil || r.db == nil {
		return service.ErrTouchPieDeviceNotFound
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE touch_pie_device_sessions
		SET status = $1, consumed_at = $2, updated_at = $2
		WHERE id = $3 AND status = $4 AND consumed_at IS NULL AND expires_at > $2
	`, service.TouchPieDeviceStatusConsumed, now, id, service.TouchPieDeviceStatusApproved)
	return touchPieRowsAffected(res, err)
}

func (r *touchPieDeviceRepository) ExpireDeviceSession(ctx context.Context, id int64, now time.Time) error {
	if r == nil || r.db == nil {
		return service.ErrTouchPieDeviceNotFound
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE touch_pie_device_sessions
		SET status = $1, updated_at = $2
		WHERE id = $3 AND consumed_at IS NULL
	`, service.TouchPieDeviceStatusExpired, now, id)
	return err
}

func (r *touchPieDeviceRepository) getDeviceSession(ctx context.Context, column string, hash string) (*service.TouchPieDeviceSession, error) {
	if r == nil || r.db == nil || hash == "" {
		return nil, service.ErrTouchPieDeviceNotFound
	}
	query := `
		SELECT id, device_code_hash, user_code_hash, status, user_id, expires_at, approved_at, consumed_at, created_at, updated_at
		FROM touch_pie_device_sessions
		WHERE ` + column + ` = $1
		LIMIT 1
	`
	session := &service.TouchPieDeviceSession{}
	var userID sql.NullInt64
	var approvedAt, consumedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, hash).Scan(
		&session.ID,
		&session.DeviceCodeHash,
		&session.UserCodeHash,
		&session.Status,
		&userID,
		&session.ExpiresAt,
		&approvedAt,
		&consumedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrTouchPieDeviceNotFound
		}
		return nil, err
	}
	if userID.Valid {
		session.UserID = &userID.Int64
	}
	if approvedAt.Valid {
		session.ApprovedAt = &approvedAt.Time
	}
	if consumedAt.Valid {
		session.ConsumedAt = &consumedAt.Time
	}
	return session, nil
}

func touchPieRowsAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrTouchPieDeviceNotFound
	}
	return nil
}
