CREATE TABLE positions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  opener_order_id BIGINT UNSIGNED NOT NULL,
  closer_order_id BIGINT UNSIGNED,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_positions_opener_order_id
    FOREIGN KEY (opener_order_id)
    REFERENCES orders(id),
  CONSTRAINT fk_positions_closer_order_id
    FOREIGN KEY (closer_order_id)
    REFERENCES orders(id)
);