package domain

import (
	"context"
	"math"

	"github.com/google/uuid"

	"github.com/ayosage/cellarkeep-server/internal/store"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

// InventoryRow is one ingredient with the stock currently on hand.
type InventoryRow struct {
	Ingredient  gen.Ingredient
	Stock       float64
	CostPerUnit *float64
}

// Shortage reports how much of one ingredient a plan needs beyond stock.
type Shortage struct {
	IngredientID uuid.UUID
	Name         string
	Required     float64
	Available    float64
	Short        float64
}

type Inventory struct{ S *store.Store }

func (i Inventory) List(ctx context.Context) ([]InventoryRow, error) {
	rows, err := i.S.Q.ListInventory(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]InventoryRow, 0, len(rows))
	for _, r := range rows {
		var cost *float64
		if r.CostPerUnit != nil {
			c := float64(*r.CostPerUnit)
			cost = &c
		}
		out = append(out, InventoryRow{
			Ingredient:  gen.Ingredient{ID: r.ID, Name: r.Name, Category: r.Category, Unit: r.Unit},
			Stock:       float64(r.Stock),
			CostPerUnit: cost,
		})
	}
	return out, nil
}

func (i Inventory) CreateIngredient(ctx context.Context, p gen.CreateIngredientParams) (gen.Ingredient, error) {
	return i.S.Q.CreateIngredient(ctx, p)
}

// AdjustStock moves stock by delta and never lets it fall below zero. It
// returns the new level.
func (i Inventory) AdjustStock(ctx context.Context, ingredientID uuid.UUID, delta float64) (float64, error) {
	var next float64
	err := i.S.WithTx(ctx, func(q *gen.Queries) error {
		item, err := q.EnsureInventoryItem(ctx, ingredientID)
		if err != nil {
			return err
		}
		next = math.Max(0, round3(float64(item.Quantity)+delta))
		return q.SetInventoryQuantity(ctx, gen.SetInventoryQuantityParams{ID: item.ID, Quantity: float32(next)})
	})
	return next, err
}

// DeductForAddition takes what is in stock, never below zero, and reports the
// shortfall. It runs inside the caller's transaction.
func DeductForAddition(ctx context.Context, q *gen.Queries, ingredientID uuid.UUID, qty float64) (deducted, short float64, err error) {
	item, err := q.EnsureInventoryItem(ctx, ingredientID)
	if err != nil {
		return 0, 0, err
	}
	have := float64(item.Quantity)
	deducted = round3(math.Min(have, qty))
	short = round3(qty - deducted)
	if deducted > 0 {
		err = q.SetInventoryQuantity(ctx, gen.SetInventoryQuantityParams{
			ID: item.ID, Quantity: float32(round3(have - deducted)),
		})
	}
	return deducted, short, err
}
