package domain_test

import (
	"context"
	"testing"

	"github.com/ayosage/cellarkeep-server/internal/domain"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

func TestAdjustAndDeduct(t *testing.T) {
	ctx := context.Background()
	s := txStore(t)
	inv := domain.Inventory{S: s}
	honey, err := inv.CreateIngredient(ctx, gen.CreateIngredientParams{
		Name: "Honey", Category: gen.IngredientCategoryFermentable, Unit: "kg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := inv.AdjustStock(ctx, honey.ID, 5); err != nil || n != 5 {
		t.Fatalf("stock %v err %v", n, err)
	}
	if n, err := inv.AdjustStock(ctx, honey.ID, -10); err != nil || n != 0 {
		t.Fatalf("stock must floor at 0, got %v err %v", n, err)
	}
	if _, err := inv.AdjustStock(ctx, honey.ID, 2); err != nil {
		t.Fatal(err)
	}
	ded, short, err := domain.DeductForAddition(ctx, s.Q, honey.ID, 3)
	if err != nil || ded != 2 || short != 1 {
		t.Fatalf("ded %v short %v err %v", ded, short, err)
	}
	rows, err := inv.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Stock != 0 || rows[0].Ingredient.Name != "Honey" {
		t.Fatalf("rows %+v", rows)
	}
}
