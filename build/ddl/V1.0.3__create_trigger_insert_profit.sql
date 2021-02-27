CREATE TRIGGER insert_profits
  AFTER INSERT ON contracts FOR EACH ROW
  INSERT INTO profits (amount)
  SELECT SUM(amount) FROM (
      SELECT increase_amount amount FROM contracts WHERE increase_currency = 'jpy'
      UNION ALL
      SELECT decrease_amount amount FROM contracts WHERE decrease_currency = 'jpy'
  ) c
;
