-- Migration: 008_seed_tma_operator
-- Seeds the built-in Telegram Mini App operator used by /tma/auth.
--
-- This operator is referenced by TELEGRAM_TMA_OPERATOR_ID (default: 1).
-- The api_key 'lava_tma_internal' is never exposed — /tma/auth uses
-- Telegram initData validation instead of HMAC middleware.

INSERT INTO operators (
    id,
    name,
    api_key,
    secret_key,
    status,
    callback_url,
    default_rtp_profile_id
)
OVERRIDING SYSTEM VALUE
VALUES (
    1,
    'Telegram Mini App',
    'lava_tma_internal',
    gen_random_uuid()::text,
    'active',
    '',
    (SELECT id FROM rtp_profiles WHERE name = 'Standard 96%' LIMIT 1)
)
ON CONFLICT (id) DO NOTHING;

-- Advance the sequence past the seeded ID so future operators don't collide
SELECT setval('operators_id_seq', GREATEST(nextval('operators_id_seq'), 2));
