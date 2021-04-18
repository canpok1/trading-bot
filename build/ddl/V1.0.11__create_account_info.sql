CREATE TABLE account_info_type (
  name VARCHAR(15),
  memo TEXT,
  PRIMARY KEY (name)
);
INSERT INTO account_info_type(name, memo) VALUES('total_jpy', '資金(JPY)');

CREATE TABLE account_info (
  type VARCHAR(15) NOT NULL,
  value DECIMAL(15,4) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (type),
  FOREIGN KEY (type) REFERENCES account_info_type(name)
);