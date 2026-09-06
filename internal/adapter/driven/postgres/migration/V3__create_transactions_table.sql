CREATE TABLE IF NOT EXISTS transactions
(
    hash         VARCHAR(64) PRIMARY KEY,
    block_height BIGINT REFERENCES blocks (height),
    sender_id    VARCHAR(64) NOT NULL,
    receiver_id  VARCHAR(64) NOT NULL,
    nonce        BIGINT      NOT NULL,
    timestamp    BIGINT      NOT NULL,
    red          BIGINT      NOT NULL DEFAULT 0,
    green        BIGINT      NOT NULL DEFAULT 0,
    blue         BIGINT      NOT NULL DEFAULT 0,
    type         SMALLINT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_transactions_block_height ON transactions (block_height);
CREATE INDEX IF NOT EXISTS idx_transactions_sender_id ON transactions (sender_id);
CREATE INDEX IF NOT EXISTS idx_transactions_receiver_id ON transactions (receiver_id);
