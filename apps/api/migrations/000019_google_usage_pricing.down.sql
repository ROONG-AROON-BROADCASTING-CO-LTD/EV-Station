UPDATE api_usage_plans
SET included_units = NULL,
    overage_price_thb = NULL
WHERE provider_id IN ('google-maps-js', 'google-places');
