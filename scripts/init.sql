-- Create database (run this manually first)
-- CREATE DATABASE coupon_system;

-- Create campaigns table
CREATE TABLE IF NOT EXISTS campaigns (
    id BIGSERIAL PRIMARY KEY,
    available_coupons INTEGER NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create coupons table
CREATE TABLE IF NOT EXISTS coupons (
    code VARCHAR(10) PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id),
    status VARCHAR(20) DEFAULT 'available',
    issued_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_coupons_campaign_id ON coupons(campaign_id);
CREATE INDEX IF NOT EXISTS idx_coupons_issued_at ON coupons(issued_at);
CREATE INDEX IF NOT EXISTS idx_campaigns_start_date ON campaigns(start_date);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for campaigns table
DROP TRIGGER IF EXISTS update_campaigns_updated_at ON campaigns;
CREATE TRIGGER update_campaigns_updated_at
    BEFORE UPDATE ON campaigns
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
