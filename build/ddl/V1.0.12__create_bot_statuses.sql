CREATE TABLE bot_statuses (
  type VARCHAR(50) NOT NULL,
  value DECIMAL(15,4) NOT NULL,
  memo TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (type)
);