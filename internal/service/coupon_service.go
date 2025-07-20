package service

import (
	"context"
	"crypto/aes"
	"encoding/binary"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jmoiron/sqlx"

	couponv1 "github.com/kkkkikiki/coupon/gen/coupon/v1"
	"github.com/kkkkikiki/coupon/internal/metrics"
	"github.com/kkkkikiki/coupon/internal/model"
	"github.com/kkkkikiki/coupon/internal/repository"
)

// CouponServer implements the coupon service
type CouponServer struct {
	postgres     *sqlx.DB
	campaignRepo *repository.CampaignRepository
	couponRepo   *repository.CouponRepository
}

// NewCouponServer creates a new CouponServer instance
func NewCouponServer(postgres *sqlx.DB) *CouponServer {
	return &CouponServer{
		postgres:     postgres,
		campaignRepo: repository.NewCampaignRepository(),
		couponRepo:   repository.NewCouponRepository(),
	}
}

// CreateCampaign creates a new coupon campaign
func (s *CouponServer) CreateCampaign(
	ctx context.Context,
	req *connect.Request[couponv1.CreateCampaignRequest],
) (*connect.Response[couponv1.CreateCampaignResponse], error) {
	// Create campaign model
	campaign := &model.Campaign{
		AvailableCoupons: req.Msg.AvailableCoupons,
		StartDate:        req.Msg.StartDate.AsTime(),
	}

	// Start transaction
	tx, err := s.postgres.BeginTxx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to begin transaction: %w", err))
	}
	defer tx.Rollback()

	// Create campaign in database (this will set campaign.ID)
	if err := s.campaignRepo.CreateCampaign(tx, campaign); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create campaign: %w", err))
	}

	// Pre-generate all coupon codes using the generated campaign ID
	couponCodes := make([]string, 0, int(req.Msg.AvailableCoupons))
	for i := 0; i < int(req.Msg.AvailableCoupons); i++ {
		// Use campaign ID + coupon index for unique generation
		code, err := s.generateSecureCoupon(campaign.ID, uint64(i))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate coupon code: %w", err))
		}
		couponCodes = append(couponCodes, code)
	}

	// Store coupons in DB only (DB-centric approach)
	if err := s.couponRepo.CreatePregeneratedCoupons(tx, campaign.ID, couponCodes); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to store coupons in DB: %w", err))
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to commit transaction: %w", err))
	}

	// Convert to protobuf response
	protoCampaign := &couponv1.Campaign{
		Id:                campaign.ID,
		AvailableCoupons:  campaign.AvailableCoupons,
		StartDate:         timestamppb.New(campaign.StartDate),
		IssuedCouponCodes: []string{}, // Initially empty
	}

	res := connect.NewResponse(&couponv1.CreateCampaignResponse{
		Campaign: protoCampaign,
	})

	return res, nil
}

// generateSecureCoupon generates AES-encrypted coupon code (always 10 characters)
func (s *CouponServer) generateSecureCoupon(campaignID int64, couponIndex uint64) (string, error) {
	// "읽기 편한" 28자 + 숫자 10개 = 38문자
	digits := []rune("0123456789")
	hanguls := []rune("가나다라마바사아자차카타파하거너더러머버서어저처커터퍼허")
	pool := append(digits, hanguls...) // 38 rune
	base := uint64(len(pool))          // 38

	// Campaign ID + Coupon Index로 고유한 시퀀스 생성
	seq := s.createUniqueSequence(campaignID, couponIndex)

	// AES 키 생성 (캠페인별 고정 키)
	key := s.generateCampaignKey(campaignID)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// ① 128-bit 평문: 상위 64bit 0, 하위 64bit = seq
	var plain [16]byte
	binary.BigEndian.PutUint64(plain[8:], seq)

	// ② AES-ECB 암호화 (한 블록)
	var cipher [16]byte
	block.Encrypt(cipher[:], plain[:])

	// ③ 필수 문자인 앞 2글자
	digit := digits[cipher[0]%10]                  // 숫자
	hang := hanguls[cipher[1]%uint8(len(hanguls))] // 한글

	// ④ 본문 8글자용 64-bit 정수
	v := binary.BigEndian.Uint64(cipher[8:])
	body := make([]rune, 8) // bodyLen = 8
	for i := 7; i >= 0; i-- {
		body[i] = pool[v%base]
		v /= base
	}

	// ⑤ 조립: 숫자1 + 한글1 + 본문8 = 정확히 10글자
	return string([]rune{digit, hang}) + string(body), nil
}

// createUniqueSequence creates a unique sequence from campaign ID and coupon index
func (s *CouponServer) createUniqueSequence(campaignID int64, couponIndex uint64) uint64 {
	// Campaign ID를 시퀀스로 변환
	campaignSeq := s.campaignIDToSequence(campaignID)

	// Campaign sequence와 coupon index를 결합하여 고유한 시퀀스 생성
	// 상위 32비트: campaign sequence, 하위 32비트: coupon index
	return (campaignSeq << 32) | couponIndex
}

// campaignIDToSequence converts campaign ID to sequence number
func (s *CouponServer) campaignIDToSequence(campaignID int64) uint64 {
	// Campaign ID가 int64이므로 직접 uint64로 변환
	return uint64(campaignID)
}

// generateCampaignKey generates a deterministic AES key for campaign
func (s *CouponServer) generateCampaignKey(campaignID int64) []byte {
	// 캠페인별 고정 키 생성 (16바이트)
	key := make([]byte, 16)
	hash := s.campaignIDToSequence(campaignID)

	// 키를 deterministic하게 생성
	for i := 0; i < 16; i++ {
		key[i] = byte((hash >> (i % 8)) ^ uint64(i*7))
	}
	return key
}

// GetCampaign gets campaign information including all issued coupon codes
func (s *CouponServer) GetCampaign(
	ctx context.Context,
	req *connect.Request[couponv1.GetCampaignRequest],
) (*connect.Response[couponv1.GetCampaignResponse], error) {
	// Get campaign with issued coupon codes from database
	campaign, couponCodes, err := s.campaignRepo.GetCampaignWithCoupons(s.postgres, req.Msg.CampaignId)
	if err != nil {
		if err.Error() == "campaign not found" {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get campaign: %w", err))
	}

	// Convert to protobuf response
	protoCampaign := &couponv1.Campaign{
		Id:                campaign.ID,
		AvailableCoupons:  campaign.AvailableCoupons,
		StartDate:         timestamppb.New(campaign.StartDate),
		IssuedCouponCodes: couponCodes,
	}

	res := connect.NewResponse(&couponv1.GetCampaignResponse{
		Campaign: protoCampaign,
	})

	return res, nil
}

// IssueCoupon requests coupon issuance on specific campaign with pre-generated coupons
func (s *CouponServer) IssueCoupon(
	ctx context.Context,
	req *connect.Request[couponv1.IssueCouponRequest],
) (*connect.Response[couponv1.IssueCouponResponse], error) {
	// Start timing for metrics
	start := time.Now()
	result := "failed"

	// Defer metric recording to ensure it's always called
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.RecordIssueCouponDuration(result, duration)
	}()

	// Get campaign from database for initial checks
	campaign, err := s.campaignRepo.GetCampaign(s.postgres, req.Msg.CampaignId)
	if err != nil {
		if err.Error() == "campaign not found" {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get campaign: %w", err))
	}

	// Check if campaign has started
	now := time.Now()
	if now.Before(campaign.StartDate) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("campaign has not started yet"))
	}

	// DB-centric approach: Use DB as single source of truth
	// Start transaction for atomic coupon reservation
	tx, err := s.postgres.BeginTxx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to begin transaction: %w", err))
	}
	defer tx.Rollback()

	// Reserve an available coupon directly from DB (atomic operation)
	couponCode, err := s.couponRepo.ReserveAvailableCoupon(tx, req.Msg.CampaignId)
	if err != nil {
		if err.Error() == "no available coupons" {
			return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("no more coupons available"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reserve coupon: %w", err))
	}

	// Mark the reserved coupon as issued
	if err := s.couponRepo.MarkCouponAsIssued(tx, couponCode); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to mark coupon as issued: %w", err))
	}

	// Commit DB transaction - this guarantees consistency
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to commit transaction: %w", err))
	}
	result = "success"

	// Create protobuf response
	coupon := &couponv1.Coupon{
		Code:       couponCode,
		CampaignId: req.Msg.CampaignId,
	}

	res := connect.NewResponse(&couponv1.IssueCouponResponse{
		Coupon: coupon,
	})

	return res, nil
}
