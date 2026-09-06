CREATE TABLE IF NOT EXISTS authorities
(
    id            VARCHAR(64) PRIMARY KEY,
    pub_key_hex   VARCHAR(128) NOT NULL,
    registered_at BIGINT       NOT NULL,
    CONSTRAINT authorities_pub_key_hex_key UNIQUE (pub_key_hex)
);
