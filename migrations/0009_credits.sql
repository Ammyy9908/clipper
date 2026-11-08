-- 0009_credits.sql: Credit balances, transaction logs, and Razorpay orders.

ALTER TABLE users ADD COLUMN IF NOT EXISTS credits_balance INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS daily_free_credits INT NOT NULL DEFAULT 10;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_refill_date DATE NOT NULL DEFAULT CURRENT_DATE;

CREATE TABLE IF NOT EXISTS credit_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    amount INT NOT NULL,
    balance_after INT NOT NULL,
    task_type TEXT NOT NULL,
    job_id UUID,
    payment_id TEXT,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS credit_transactions_user_created_idx ON credit_transactions (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS payment_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    razorpay_order_id TEXT UNIQUE NOT NULL,
    razorpay_payment_id TEXT,
    pack_id TEXT NOT NULL,
    credits INT NOT NULL,
    amount INT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'INR',
    status TEXT NOT NULL DEFAULT 'created',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS payment_orders_user_idx ON payment_orders (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS payment_orders_rzp_idx ON payment_orders (razorpay_order_id);
