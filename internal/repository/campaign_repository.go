package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kkkkikiki/coupon/internal/model"
)

// DBExecutor interface for database operations (can be *sqlx.DB or *sqlx.Tx)
type DBExecutor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
}

// CampaignRepository handles campaign data operations
type CampaignRepository struct {
	// DB-only repository - no Redis dependencies
}

// NewCampaignRepository creates a new campaign repository
func NewCampaignRepository() *CampaignRepository {
	return &CampaignRepository{}
}

// CreateCampaign creates a new campaign
func (r *CampaignRepository) CreateCampaign(db DBExecutor, campaign *model.Campaign) error {
	query := `
		INSERT INTO campaigns (available_coupons, start_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	now := time.Now()
	campaign.CreatedAt = now
	campaign.UpdatedAt = now

	err := db.Get(&campaign.ID, query,
		campaign.AvailableCoupons, campaign.StartDate, campaign.CreatedAt, campaign.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create campaign: %w", err)
	}

	return nil
}

// GetCampaign retrieves a campaign by ID
func (r *CampaignRepository) GetCampaign(db DBExecutor, id int64) (*model.Campaign, error) {
	query := `
		SELECT id, available_coupons, start_date, created_at, updated_at
		FROM campaigns
		WHERE id = $1
	`

	var campaign model.Campaign
	err := db.Get(&campaign, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("campaign not found")
		}
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	return &campaign, nil
}

// GetCampaignWithCoupons retrieves a campaign with all issued coupon codes
func (r *CampaignRepository) GetCampaignWithCoupons(db DBExecutor, campaignID int64) (*model.Campaign, []string, error) {
	campaign, err := r.GetCampaign(db, campaignID)
	if err != nil {
		return nil, nil, err
	}

	// Get only successfully issued coupon codes (status = 'issued')
	query := `
		SELECT code
		FROM coupons
		WHERE campaign_id = $1 AND status = 'issued'
		ORDER BY issued_at ASC
	`

	var couponCodes []string
	err = db.Select(&couponCodes, query, campaignID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get coupon codes: %w", err)
	}

	return campaign, couponCodes, nil
}
