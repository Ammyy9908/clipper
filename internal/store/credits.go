package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

type UserCredits struct {
	CreditsBalance   int       `json:"credits_balance"`
	DailyFreeCredits int       `json:"daily_free_credits"`
	TotalAvailable   int       `json:"total_available"`
	LastRefillDate   time.Time `json:"last_refill_date"`
}

type CreditTransaction struct {
	ID           uuid.UUID  `json:"id"`
	UserID       string     `json:"user_id"`
	Amount       int        `json:"amount"`
	BalanceAfter int        `json:"balance_after"`
	TaskType     string     `json:"task_type"`
	JobID        *uuid.UUID `json:"job_id,omitempty"`
	PaymentID    *string    `json:"payment_id,omitempty"`
	Description  string     `json:"description"`
	CreatedAt    time.Time  `json:"created_at"`
}

type PaymentOrder struct {
	ID                uuid.UUID  `json:"id"`
	UserID            string     `json:"user_id"`
	RazorpayOrderID   string     `json:"razorpay_order_id"`
	RazorpayPaymentID *string    `json:"razorpay_payment_id,omitempty"`
	PackID            string     `json:"pack_id"`
	Credits           int        `json:"credits"`
	Amount            int        `json:"amount"` // in paise
	Currency          string     `json:"currency"`
	Status            string     `json:"status"` // "created", "paid", "failed"
	CreatedAt         time.Time  `json:"created_at"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
}

// GetOrRefillCredits fetches the user's credits, automatically replenishing
// daily free credits if the local day has rolled over.
func (s *Store) GetOrRefillCredits(ctx context.Context, userID string, defaultDailyFree int) (UserCredits, error) {
	if defaultDailyFree <= 0 {
		defaultDailyFree = 10
	}
	var uc UserCredits
	var lastRefill time.Time

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uc, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`SELECT credits_balance, daily_free_credits, last_refill_date
		 FROM users
		 WHERE id = $1
		 FOR UPDATE`, userID).Scan(&uc.CreditsBalance, &uc.DailyFreeCredits, &lastRefill)

	if errors.Is(err, pgx.ErrNoRows) {
		// New user or anon
		uc.DailyFreeCredits = defaultDailyFree
		uc.CreditsBalance = 0
		uc.TotalAvailable = defaultDailyFree
		return uc, nil
	}
	if err != nil {
		return uc, err
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	if lastRefill.UTC().Before(today) {
		uc.DailyFreeCredits = defaultDailyFree
		uc.LastRefillDate = today
		_, err = tx.Exec(ctx,
			`UPDATE users
			 SET daily_free_credits = $1, last_refill_date = CURRENT_DATE
			 WHERE id = $2`, defaultDailyFree, userID)
		if err != nil {
			return uc, err
		}
	} else {
		uc.LastRefillDate = lastRefill
	}

	uc.TotalAvailable = uc.CreditsBalance + uc.DailyFreeCredits
	if err := tx.Commit(ctx); err != nil {
		return uc, err
	}
	return uc, nil
}

// ConsumeCreditsTx charges a user for a task, deducting from daily free credits
// first, then from purchased credits balance.
func (s *Store) ConsumeCreditsTx(ctx context.Context, tx pgx.Tx, userID string, amount int, taskType string, jobID *uuid.UUID, desc string, defaultDailyFree int) (int, error) {
	if amount <= 0 {
		return 0, nil
	}
	if defaultDailyFree <= 0 {
		defaultDailyFree = 10
	}

	var balance, dailyFree int
	var lastRefill time.Time

	err := tx.QueryRow(ctx,
		`SELECT credits_balance, daily_free_credits, last_refill_date
		 FROM users
		 WHERE id = $1
		 FOR UPDATE`, userID).Scan(&balance, &dailyFree, &lastRefill)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	if lastRefill.UTC().Before(today) {
		dailyFree = defaultDailyFree
	}

	total := balance + dailyFree
	if total < amount {
		return total, ErrInsufficientCredits
	}

	freeDeduct := amount
	if freeDeduct > dailyFree {
		freeDeduct = dailyFree
	}
	paidDeduct := amount - freeDeduct

	dailyFree -= freeDeduct
	balance -= paidDeduct
	totalAfter := balance + dailyFree

	_, err = tx.Exec(ctx,
		`UPDATE users
		 SET credits_balance = $1, daily_free_credits = $2, last_refill_date = CURRENT_DATE
		 WHERE id = $3`, balance, dailyFree, userID)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO credit_transactions (user_id, amount, balance_after, task_type, job_id, description)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, -amount, totalAfter, taskType, jobID, desc)
	if err != nil {
		return 0, err
	}

	return totalAfter, nil
}

// CreateJobWithCredits charges a user for a task and creates the job inside the same database transaction.
func (s *Store) CreateJobWithCredits(ctx context.Context, j Job, cost int, taskType, desc string, defaultDailyFree int) error {
	if j.UserID == nil || *j.UserID == "" {
		return errors.New("user_id required for credits")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = s.ConsumeCreditsTx(ctx, tx, *j.UserID, cost, taskType, &j.ID, desc, defaultDailyFree)
	if err != nil {
		return err
	}

	if j.TextPosition == "" {
		j.TextPosition = "top"
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO jobs (id, source_type, source_key, source_url, format, audio_only,
		                   segment_seconds, aspect, mode, status, user_id,
		                   text_overlay, show_part_counter, text_position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		j.ID, j.SourceType, j.SourceKey, j.SourceURL, j.Format, j.AudioOnly,
		j.SegmentSeconds, j.Aspect, j.Mode, StatusQueued, j.UserID,
		j.TextOverlay, j.ShowPartCounter, j.TextPosition)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// CreatePaymentOrder logs a new pending checkout order.
func (s *Store) CreatePaymentOrder(ctx context.Context, o PaymentOrder) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO payment_orders (id, user_id, razorpay_order_id, pack_id, credits, amount, currency, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		o.ID, o.UserID, o.RazorpayOrderID, o.PackID, o.Credits, o.Amount, o.Currency, "created")
	return err
}

// CompletePaymentOrder marks an order paid and tops up the user's credit balance.
// It is idempotent and safe against duplicate webhook/client calls.
func (s *Store) CompletePaymentOrder(ctx context.Context, razorpayOrderID, paymentID string) (*PaymentOrder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var o PaymentOrder
	err = tx.QueryRow(ctx,
		`SELECT id, user_id, razorpay_order_id, pack_id, credits, amount, currency, status, created_at
		 FROM payment_orders
		 WHERE razorpay_order_id = $1
		 FOR UPDATE`, razorpayOrderID).Scan(&o.ID, &o.UserID, &o.RazorpayOrderID, &o.PackID, &o.Credits, &o.Amount, &o.Currency, &o.Status, &o.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if o.Status == "paid" {
		// Already credited
		return &o, nil
	}

	now := time.Now().UTC()
	o.PaidAt = &now
	o.RazorpayPaymentID = &paymentID
	o.Status = "paid"

	_, err = tx.Exec(ctx,
		`UPDATE payment_orders
		 SET status = 'paid', razorpay_payment_id = $1, paid_at = now()
		 WHERE id = $2`, paymentID, o.ID)
	if err != nil {
		return nil, err
	}

	var newBalance, dailyFree int
	err = tx.QueryRow(ctx,
		`UPDATE users
		 SET credits_balance = credits_balance + $1
		 WHERE id = $2
		 RETURNING credits_balance, daily_free_credits`, o.Credits, o.UserID).Scan(&newBalance, &dailyFree)
	if err != nil {
		return nil, err
	}

	desc := fmt.Sprintf("Purchased %d credits (%s)", o.Credits, o.PackID)
	_, err = tx.Exec(ctx,
		`INSERT INTO credit_transactions (user_id, amount, balance_after, task_type, payment_id, description)
		 VALUES ($1, $2, $3, 'purchase', $4, $5)`,
		o.UserID, o.Credits, newBalance+dailyFree, paymentID, desc)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &o, nil
}

// RefundJobCredits returns charged credits if a job failed or was purged.
func (s *Store) RefundJobCredits(ctx context.Context, jobID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var (
		userID string
		amount int
	)
	err = tx.QueryRow(ctx,
		`SELECT user_id, amount
		 FROM credit_transactions
		 WHERE job_id = $1 AND amount < 0
		 LIMIT 1`, jobID).Scan(&userID, &amount)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil // Not charged or no credits used
	}
	if err != nil {
		return err
	}

	// Check if already refunded
	var existingRefund int
	err = tx.QueryRow(ctx,
		`SELECT count(*) FROM credit_transactions WHERE job_id = $1 AND amount > 0`, jobID).Scan(&existingRefund)
	if err != nil {
		return err
	}
	if existingRefund > 0 {
		return nil // Already refunded
	}

	refundAmount := -amount // positive
	var newBalance, dailyFree int
	err = tx.QueryRow(ctx,
		`UPDATE users
		 SET credits_balance = credits_balance + $1
		 WHERE id = $2
		 RETURNING credits_balance, daily_free_credits`, refundAmount, userID).Scan(&newBalance, &dailyFree)
	if err != nil {
		return err
	}

	desc := fmt.Sprintf("Refund for failed job %s", jobID.String())
	_, err = tx.Exec(ctx,
		`INSERT INTO credit_transactions (user_id, amount, balance_after, task_type, job_id, description)
		 VALUES ($1, $2, $3, 'refund', $4, $5)`,
		userID, refundAmount, newBalance+dailyFree, jobID, desc)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ListCreditTransactions returns recent transactions for a user.
func (s *Store) ListCreditTransactions(ctx context.Context, userID string, limit int) ([]CreditTransaction, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, amount, balance_after, task_type, job_id, payment_id, description, created_at
		 FROM credit_transactions
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CreditTransaction
	for rows.Next() {
		var t CreditTransaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.BalanceAfter, &t.TaskType, &t.JobID, &t.PaymentID, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}
