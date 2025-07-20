package model

import (
	"time"
)

// Campaign represents a coupon campaign in the database
type Campaign struct {
	ID               int64     `db:"id" json:"id"`
	AvailableCoupons int32     `db:"available_coupons" json:"available_coupons"`
	StartDate        time.Time `db:"start_date" json:"start_date"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// Coupon represents an issued coupon in the database
type Coupon struct {
	Code       string    `db:"code" json:"code"`
	CampaignID int64     `db:"campaign_id" json:"campaign_id"`
	Status     string    `db:"status" json:"status"`        // 'available' or 'issued'
	IssuedAt   time.Time `db:"issued_at" json:"issued_at"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
