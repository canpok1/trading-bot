ALTER TABLE bot_statuses
ADD pair VARCHAR(15) NOT NULL COMMENT 'all or pair string (e.g.: btc_jpy)' AFTER bot_name ,
DROP PRIMARY KEY,
ADD PRIMARY KEY (bot_name, pair, type);
