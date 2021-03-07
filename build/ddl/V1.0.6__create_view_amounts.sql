CREATE VIEW amounts_view AS
SELECT p.id position_id, oc.decrease_amount decrease_jpy, cc.increase_amount increase_jpy
FROM positions p
  LEFT OUTER JOIN contracts oc ON p.opener_order_id = oc.order_id
  LEFT OUTER JOIN contracts cc ON p.closer_order_id = cc.order_id
;