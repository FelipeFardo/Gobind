-- name: CreatedBid :one
INSERT INTO bids(
  product_id, bidder_id, bid_amount_in_cents
) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetBidsByProductId :many
SELECT * FROM bids
WHERE product_id = $1
ORDER BY bid_amount_in_cents DESC;

-- name: GetHighestBidByProductId :one
SELECT * FROM bids
WHERE product_id = $1
ORDER BY bid_amount_in_cents DESC
LIMIT 1;