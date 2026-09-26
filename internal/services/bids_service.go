package services

import (
	"context"
	"errors"

	"github.com/felipefardo/gobid/internal/store/pgstore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BidsService struct {
	pool    *pgxpool.Pool
	queries *pgstore.Queries
}

func NewBidsService(pool *pgxpool.Pool) BidsService {
	return BidsService{
		pool:    pool,
		queries: pgstore.New(pool),
	}
}

var ErrBidIsTooLow = errors.New("the bid value is too low")

func (bs *BidsService) Placebid(ctx context.Context, product_id, bidder_id uuid.UUID, amount int32) (pgstore.Bid, error) {
	// amount > previous_amount
	// amount > baseprice

	product, err := bs.queries.GetProductById(ctx, product_id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgstore.Bid{}, err
		}
	}

	highestBid, err := bs.queries.GetHighestBidByProductId(ctx, product_id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return pgstore.Bid{}, err
		}
	}

	if product.BasepriceInCents >= amount || highestBid.BidAmountInCents >= amount {
		return pgstore.Bid{}, ErrBidIsTooLow
	}

	highestBid, err = bs.queries.CreatedBid(ctx, pgstore.CreatedBidParams{
		ProductID:        product_id,
		BidderID:         bidder_id,
		BidAmountInCents: amount,
	})
	if err != nil {
		return pgstore.Bid{}, err
	}

	return highestBid, nil
}
