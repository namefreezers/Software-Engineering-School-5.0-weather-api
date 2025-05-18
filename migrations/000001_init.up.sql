-- 1. Enable pgcrypto extension for gen_random_uuid()
CREATE
EXTENSION IF NOT EXISTS pgcrypto;

-- 2. Create the subscriptions table
CREATE TABLE subscriptions
(
    id                SERIAL PRIMARY KEY,
    email             VARCHAR(255) NOT NULL UNIQUE,
    city              VARCHAR(100) NOT NULL,
    frequency         VARCHAR(10)  NOT NULL
        CHECK (frequency IN ('hourly', 'daily')),
    confirmed         BOOLEAN      NOT NULL DEFAULT FALSE,
    confirm_token     UUID UNIQUE           DEFAULT gen_random_uuid(),
    unsubscribe_token UUID UNIQUE  NOT NULL DEFAULT gen_random_uuid(),
    scheduled_minute  SMALLINT     NOT NULL DEFAULT 0
        CHECK (scheduled_minute BETWEEN 0 AND 59),
    scheduled_hour    SMALLINT     NOT NULL DEFAULT 0
        CHECK (scheduled_hour BETWEEN 0 AND 23),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- 3. Partial indexes for scheduler lookups
CREATE INDEX idx_subs_hourly
    ON subscriptions (scheduled_minute) WHERE confirmed = TRUE AND frequency = 'hourly';

CREATE INDEX idx_subs_daily
    ON subscriptions (scheduled_hour, scheduled_minute) WHERE confirmed = TRUE AND frequency = 'daily';
