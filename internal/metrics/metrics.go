package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// IssueCouponDuration tracks the latency of coupon issuance
	IssueCouponDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "coupon_issue_duration_seconds",
			Help: "Duration of coupon issuance requests in seconds",
			Buckets: []float64{
				0.001, // 1ms
				0.005, // 5ms
				0.01,  // 10ms
				0.025, // 25ms
				0.05,  // 50ms
				0.1,   // 100ms
				0.25,  // 250ms
				0.5,   // 500ms
				1.0,   // 1s
				2.5,   // 2.5s
				5.0,   // 5s
				10.0,  // 10s
			},
		},
		[]string{"status"}, // success or failure
	)
)

// RecordIssueCouponDuration records the duration of a coupon issuance request
func RecordIssueCouponDuration(status string, duration float64) {
	IssueCouponDuration.WithLabelValues(status).Observe(duration)
}
