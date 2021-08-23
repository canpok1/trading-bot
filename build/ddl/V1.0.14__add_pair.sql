ALTER TABLE bot_statuses
ADD pair VARCHAR(15) DEFAULT NULL AFTER bot_name,
DROP PRIMARY KEY,
ADD PRIMARY KEY (bot_name, pair, type);
