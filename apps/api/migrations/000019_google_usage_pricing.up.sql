-- Google publishes these global USD tiers. We use 36 THB/USD only for the
-- dashboard estimate; the actual Cloud Billing invoice remains authoritative.
UPDATE api_usage_plans
SET included_units = 10000,
    overage_price_thb = 0.2520
WHERE provider_id = 'google-maps-js';

UPDATE api_usage_plans
SET included_units = 5000,
    overage_price_thb = 1.1520
WHERE provider_id = 'google-places';
