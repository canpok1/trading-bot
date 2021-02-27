DROP TRIGGER insert_profits;

CREATE TRIGGER insert_profits
  AFTER INSERT ON contracts FOR EACH ROW
  INSERT INTO profits (amount)
  SELECT SUM(amount) amount
  FROM (
    SELECT p.id position_id, a2.amount + a1.amount amount
    FROM positions p
      INNER JOIN (
        SELECT order_id, SUM(amount) amount
        FROM (
          SELECT order_id, increase_amount amount FROM contracts WHERE increase_currency = 'jpy'
          UNION ALL
          SELECT order_id, decrease_amount amount FROM contracts WHERE decrease_currency = 'jpy'
        ) a
        GROUP BY order_id
      ) a1 ON p.opener_order_id = a1.order_id
      INNER JOIN (
        SELECT order_id, SUM(amount) amount
        FROM (
          SELECT order_id, increase_amount amount FROM contracts WHERE increase_currency = 'jpy'
          UNION ALL
          SELECT order_id, decrease_amount amount FROM contracts WHERE decrease_currency = 'jpy'
        ) a
        GROUP BY order_id
      ) a2 ON p.closer_order_id = a2.order_id

    UNION ALL
    
    SELECT 0 position_id, 0 amount
  ) p
;