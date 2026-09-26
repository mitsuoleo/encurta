ALTER TABLE clicks
    ADD COLUMN device VARCHAR(16) NOT NULL DEFAULT 'unknown',
    ADD COLUMN browser VARCHAR(16) NOT NULL DEFAULT 'unknown';

UPDATE clicks SET
    device = CASE
        WHEN user_agent ILIKE '%ipad%' OR user_agent ILIKE '%tablet%' THEN 'tablet'
        WHEN user_agent ILIKE '%mobi%' OR user_agent ILIKE '%android%' OR user_agent ILIKE '%iphone%' THEN 'mobile'
        WHEN user_agent IS NULL OR user_agent = '' THEN 'unknown'
        ELSE 'desktop'
    END,
    browser = CASE
        WHEN user_agent ILIKE '%edg/%' THEN 'edge'
        WHEN user_agent ILIKE '%chrome/%' AND user_agent NOT ILIKE '%edg/%' THEN 'chrome'
        WHEN user_agent ILIKE '%firefox/%' THEN 'firefox'
        WHEN user_agent ILIKE '%safari/%' AND user_agent NOT ILIKE '%chrome/%' THEN 'safari'
        WHEN user_agent IS NULL OR user_agent = '' THEN 'unknown'
        ELSE 'other'
    END;
