package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// CouponRepository handles coupon data operations
type CouponRepository struct {
	// DB-only repository - no Redis dependencies
}

// NewCouponRepository creates a new coupon repository
func NewCouponRepository() *CouponRepository {
	return &CouponRepository{}
}

// MarkCouponAsIssued updates coupon status from 'available' to 'issued'
func (r *CouponRepository) MarkCouponAsIssued(db DBExecutor, couponCode string) error {
	query := `
		UPDATE coupons 
		SET status = 'issued', issued_at = $1 
		WHERE code = $2 AND status = 'available'
	`

	now := time.Now()
	result, err := db.Exec(query, now, couponCode)
	if err != nil {
		return fmt.Errorf("failed to mark coupon as issued: %w", err)
	}

	// Check if any row was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("coupon not found or already issued")
	}

	return nil
}

// ReserveAvailableCoupon finds and reserves an available coupon using SELECT FOR UPDATE
func (r *CouponRepository) ReserveAvailableCoupon(tx *sqlx.Tx, campaignID int64) (string, error) {
	query := `
		SELECT code 
		FROM coupons 
		WHERE campaign_id = $1 AND status = 'available' 
		ORDER BY created_at ASC 
		LIMIT 1 
		FOR UPDATE SKIP LOCKED
	`

	var couponCode string
	err := tx.Get(&couponCode, query, campaignID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no available coupons")
		}
		return "", fmt.Errorf("failed to reserve coupon: %w", err)
	}

	return couponCode, nil
}

// CreatePregeneratedCoupons creates multiple coupons in batch within existing transaction
func (r *CouponRepository) CreatePregeneratedCoupons(tx *sqlx.Tx, campaignID int64, couponCodes []string) error {
	now := time.Now()

	// 배치 크기 설정 (PostgreSQL 파라미터 제한 고려)
	batchSize := 1000

	for i := 0; i < len(couponCodes); i += batchSize {
		end := i + batchSize
		if end > len(couponCodes) {
			end = len(couponCodes)
		}

		batch := couponCodes[i:end]
		if err := r.insertCouponBatch(tx, campaignID, batch, now); err != nil {
			return fmt.Errorf("failed to insert coupon batch: %w", err)
		}
	}

	return nil
}

// insertCouponBatch inserts a batch of coupons using a single query
func (r *CouponRepository) insertCouponBatch(tx *sqlx.Tx, campaignID int64, codes []string, createdAt time.Time) error {
	if len(codes) == 0 {
		return nil
	}

	// VALUES 절을 동적으로 생성
	valuesClause := make([]string, len(codes))
	args := make([]interface{}, 0, len(codes)*4)

	for i, code := range codes {
		valuesClause[i] = fmt.Sprintf("($%d, $%d, $%d, $%d)",
			i*4+1, i*4+2, i*4+3, i*4+4)
		args = append(args, code, campaignID, "available", createdAt)
	}

	query := fmt.Sprintf(`
		INSERT INTO coupons (code, campaign_id, status, created_at)
		VALUES %s
	`, strings.Join(valuesClause, ", "))

	_, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute batch insert: %w", err)
	}

	return nil
}
