CREATE TABLE markets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  pair VARCHAR(15) NOT NULL,
  store_rate_avg DECIMAL(15,4) NOT NULL,
  ex_rate_sell DECIMAL(15,4) NOT NULL,
  ex_rate_buy DECIMAL(15,4) NOT NULL,
  ex_volume_sell DECIMAL(15,4) NOT NULL,
  ex_volume_buy DECIMAL(15,4) NOT NULL,
  recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
