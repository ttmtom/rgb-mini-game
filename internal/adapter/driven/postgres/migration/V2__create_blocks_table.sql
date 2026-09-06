CREATE TABLE IF NOT EXISTS blocks
(
    height      BIGINT PRIMARY KEY,
    hash        VARCHAR(64) NOT NULL,
    prev_hash   VARCHAR(64) NOT NULL,
    merkle_root VARCHAR(64) NOT NULL,
    timestamp   BIGINT      NOT NULL,
    nonce       BIGINT      NOT NULL DEFAULT 0,
    difficulty  SMALLINT    NOT NULL DEFAULT 0,
    CONSTRAINT blocks_hash_key UNIQUE (hash)
);
