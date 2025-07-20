package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/timestamppb"

	"connectrpc.com/connect"
	couponv1 "github.com/kkkkikiki/coupon/gen/coupon/v1"
	"github.com/kkkkikiki/coupon/gen/coupon/v1/couponv1connect"
)

// PerfResult gathers aggregated metrics for the test run.
// Atomic counters are used to avoid lock‑contention on hot paths.
// LatencySum & P95Latency are in nanoseconds.
//
// P95Latency is maintained via a lightweight reservoir sampler.
type PerfResult struct {
	TotalRequests int64
	SuccessCount  int64
	ErrorCount    int64
	LatencySum    int64
	P95Latency    int64
}

const (
	fixedWorkers    = 50
	fixedRPSTarget  = 700
	fixedDuration   = 30 * time.Second
	defaultTimeout  = 30 * time.Second
	fixedCoupons    = 50000
	fixedCreateCamp = true
)

func main() {
	// ─── Fixed Configuration ─────────────────────────────────────
	campaignIDStr := ""
	rps := fixedRPSTarget
	duration := fixedDuration
	workers := fixedWorkers
	createCampaign := fixedCreateCamp
	coupons := fixedCoupons

	// ─── HTTP Client & Transport ─────────────────────────────────
	transport := &http.Transport{
		MaxIdleConns:        workers * 4,
		MaxIdleConnsPerHost: workers * 4,
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   defaultTimeout,
	}

	// ─── Campaign handling ───────────────────────────────────────
	var campaignID int64
	var err error
	if campaignIDStr == "" || createCampaign {
		campaignID, err = createNewCampaign(httpClient, coupons)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create campaign: %v\n", err)
			os.Exit(1)
		}
		campaignIDStr = strconv.FormatInt(campaignID, 10)
		fmt.Printf("✅ 새 캠페인 생성됨: ID %s (%d개 쿠폰)\n", campaignIDStr, coupons)
	} else {
		campaignID, err = strconv.ParseInt(campaignIDStr, 10, 64)
		if err != nil || campaignID <= 0 {
			fmt.Fprintf(os.Stderr, "invalid campaign id: %q\n", campaignIDStr)
			os.Exit(1)
		}
	}

	client := couponv1connect.NewCouponServiceClient(httpClient, "http://localhost")

	// ─── Banner ──────────────────────────────────────────────────
	fmt.Println("==========================================")
	fmt.Println("🚀 Go 고성능 부하 테스트 클라이언트 (uniform)")
	fmt.Println("==========================================")
	fmt.Printf("캠페인 ID  : %s\n", campaignIDStr)
	fmt.Printf("RPS   : %d\n", rps)
	fmt.Printf("테스트 시간: %v\n", duration)
	fmt.Println("==========================================")

	// ─── Rate limiter & context ─────────────────────────────────
	burst := rps / workers
	if burst < 1 {
		burst = 1
	}
	limiter := rate.NewLimiter(rate.Limit(rps), burst)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var result PerfResult
	var wg sync.WaitGroup

	// latencyChan collects latencies for P95 estimation.
	latencyChan := make(chan time.Duration, 4096)
	go trackP95(latencyChan, &result)

	// ─── Workers ────────────────────────────────────────────────
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if err := limiter.Wait(ctx); err != nil { // context cancelled → exit
					return
				}
				doRequest(ctx, client, campaignID, &result, latencyChan)
			}
		}()
	}

	start := time.Now()
	<-ctx.Done() // wait for duration

	// ─── Cleanup ────────────────────────────────────────────────
	wg.Wait()
	close(latencyChan)

	totalDur := time.Since(start)

	// ─── Report ─────────────────────────────────────────────────
	fmt.Println("==========================================")
	fmt.Println("📊 성능 테스트 결과")
	fmt.Println("==========================================")
	fmt.Printf("테스트 시간        : %.2f초\n", totalDur.Seconds())
	fmt.Printf("총 요청 수         : %d\n", result.TotalRequests)
	fmt.Printf("성공한 요청        : %d\n", result.SuccessCount)
	fmt.Printf("실패한 요청        : %d\n", result.ErrorCount)

	actualRPS := float64(result.SuccessCount) / totalDur.Seconds()
	successRate := float64(result.SuccessCount) / float64(result.TotalRequests) * 100

	var avgLatency time.Duration
	if result.SuccessCount > 0 {
		avgLatency = time.Duration(result.LatencySum / result.SuccessCount)
	}

	fmt.Printf("실제 RPS           : %.2f\n", actualRPS)
	fmt.Printf("성공률             : %.2f%%\n", successRate)
	fmt.Printf("평균 레이턴시      : %v\n", avgLatency)
	fmt.Printf("P95 레이턴시       : %v\n", time.Duration(result.P95Latency))

	fmt.Printf("⚠️  현재 성능: %.2f RPS\n", actualRPS)

	fmt.Println("==========================================")

	// ─── Data Consistency Check ─────────────────────────────────
	fmt.Println("==========================================")
	fmt.Println("🔍 데이터 정합성 검증")
	fmt.Println("==========================================")

	if err := verifyDataConsistency(httpClient, campaignID, result.SuccessCount); err != nil {
		fmt.Printf("❌ 정합성 검증 실패: %v\n", err)
	} else {
		fmt.Println("✅ 데이터 정합성 확인 완료")
	}
	fmt.Println("==========================================")
}

// createNewCampaign creates a new campaign with the specified number of coupons.
func createNewCampaign(httpClient *http.Client, coupons int) (int64, error) {
	client := couponv1connect.NewCouponServiceClient(httpClient, "http://localhost")

	startTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	req := connect.NewRequest(&couponv1.CreateCampaignRequest{
		AvailableCoupons: int32(coupons),
		StartDate:        timestamppb.New(startTime),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.CreateCampaign(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("create campaign failed: %w", err)
	}
	if resp.Msg.Campaign == nil {
		return 0, fmt.Errorf("campaign response is nil")
	}
	return resp.Msg.Campaign.Id, nil
}

// doRequest performs a single IssueCoupon RPC and collects metrics.
func doRequest(parent context.Context, client couponv1connect.CouponServiceClient, campaignID int64, result *PerfResult, latencyChan chan<- time.Duration) {
	// Use independent context to avoid cancellation when test ends
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req := connect.NewRequest(&couponv1.IssueCouponRequest{CampaignId: campaignID})

	start := time.Now()
	atomic.AddInt64(&result.TotalRequests, 1)

	resp, err := client.IssueCoupon(ctx, req)
	latency := time.Since(start)

	if err != nil {
		atomic.AddInt64(&result.ErrorCount, 1)
		return
	}
	if resp.Msg.GetCoupon() != nil && resp.Msg.Coupon.Code != "" {
		atomic.AddInt64(&result.SuccessCount, 1)
		atomic.AddInt64(&result.LatencySum, latency.Nanoseconds())
		select {
		case latencyChan <- latency:
		default:
		}
	} else {
		atomic.AddInt64(&result.ErrorCount, 1)
	}
}

// trackP95 maintains a best‑effort rolling P95 latency estimation.
func trackP95(latencies <-chan time.Duration, result *PerfResult) {
	const size = 1000
	buf := make([]int64, 0, size)

	for lat := range latencies {
		if len(buf) < size {
			buf = append(buf, lat.Nanoseconds())
		} else {
			// Replace random element (simple reservoir sampling)
			if idx := time.Now().UnixNano() % int64(size); idx < int64(size/10) {
				buf[idx] = lat.Nanoseconds()
			}
		}

		// Update P95 periodically
		if len(buf) >= 100 && len(buf)%100 == 0 {
			copyBuf := make([]int64, len(buf))
			copy(copyBuf, buf)
			quickSort(copyBuf)
			p95Index := int(float64(len(copyBuf)) * 0.95)
			if p95Index >= len(copyBuf) {
				p95Index = len(copyBuf) - 1
			}
			atomic.StoreInt64(&result.P95Latency, copyBuf[p95Index])
		}
	}
}

// quickSort sorts the array in ascending order
func quickSort(arr []int64) {
	if len(arr) < 2 {
		return
	}

	left, right := 0, len(arr)-1
	pivot := len(arr) / 2

	arr[pivot], arr[right] = arr[right], arr[pivot]

	for i := range arr {
		if arr[i] < arr[right] {
			arr[left], arr[i] = arr[i], arr[left]
			left++
		}
	}

	arr[left], arr[right] = arr[right], arr[left]

	quickSort(arr[:left])
	quickSort(arr[left+1:])
}

// verifyDataConsistency checks if the issued coupon count matches the database state
func verifyDataConsistency(httpClient *http.Client, campaignID int64, expectedIssued int64) error {
	client := couponv1connect.NewCouponServiceClient(httpClient, "http://localhost")

	req := connect.NewRequest(&couponv1.GetCampaignRequest{
		CampaignId: campaignID,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.GetCampaign(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get campaign: %w", err)
	}

	if resp.Msg.Campaign == nil {
		return fmt.Errorf("campaign not found")
	}

	campaign := resp.Msg.Campaign
	actualIssued := int64(len(campaign.IssuedCouponCodes))
	totalCoupons := int64(campaign.AvailableCoupons)

	fmt.Printf("캠페인 ID          : %d\n", campaignID)
	fmt.Printf("전체 쿠폰 수       : %d\n", totalCoupons)
	fmt.Printf("발급된 쿠폰 (DB)   : %d\n", actualIssued)
	fmt.Printf("발급된 쿠폰 (테스트): %d\n", expectedIssued)
	fmt.Printf("남은 쿠폰 수       : %d\n", totalCoupons-actualIssued)

	if actualIssued != expectedIssued {
		return fmt.Errorf("데이터 불일치: DB=%d, 테스트=%d, 차이=%d",
			actualIssued, expectedIssued, actualIssued-expectedIssued)
	}

	// Additional checks
	if actualIssued > totalCoupons {
		return fmt.Errorf("over-issuance 발생: 발급=%d > 전체=%d", actualIssued, totalCoupons)
	}

	if actualIssued < 0 {
		return fmt.Errorf("음수 발급 수: %d", actualIssued)
	}

	return nil
}
