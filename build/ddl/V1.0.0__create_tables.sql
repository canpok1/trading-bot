CREATE TABLE orders (
  id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  order_type TINYINT NOT NULL COMMENT '0:buy 1:sell 2:market_buy 3:market_sell',
  pair VARCHAR(15) NOT NULL,
  amount DECIMAL(10,10) NOT NULL,
  rate DECIMAL(10,10),
  stop_loss_rate DECIMAL(10,10),
  status TINYINT NOT NULL DEFAULT 0 COMMENT '0:open 1:closed 2:canceled',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE contracts (
  id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  order_id BIGINT UNSIGNED NOT NULL,
  side TINYINT NOT NULL COMMENT '0:buy 1:sell',
  increase_currency VARCHAR(10) NOT NULL,
  increase_amount DECIMAL(10,10) NOT NULL,
  decrease_currency VARCHAR(10) NOT NULL,
  decrease_amount DECIMAL(10,10) NOT NULL,
  fee_currency VARCHAR(10) NOT NULL,
  fee_amount DECIMAL(10,10) NOT NULL,
  liquidity TINYINT NOT NULL COMMENT '0:Taker 1:Maker',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_contracts_order_id
    FOREIGN KEY (order_id)
    REFERENCES orders(id)
);
