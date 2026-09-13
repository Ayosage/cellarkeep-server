package api

import (
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

func (h *Handlers) CreateIngredient(w http.ResponseWriter, r *http.Request) {
	var in IngredientInput
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	ing, err := h.inv.CreateIngredient(r.Context(), gen.CreateIngredientParams{
		Name: in.Name, Category: gen.IngredientCategory(in.Category), Unit: in.Unit,
	})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ingredientJSON(ing))
}

func ingredientJSON(i gen.Ingredient) Ingredient {
	return Ingredient{Id: i.ID, Name: i.Name, Category: IngredientCategory(i.Category), Unit: i.Unit}
}

func (h *Handlers) ListInventory(w http.ResponseWriter, r *http.Request) {
	rows, err := h.inv.List(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	out := make([]InventoryRow, 0, len(rows))
	for _, row := range rows {
		item := InventoryRow{Ingredient: ingredientJSON(row.Ingredient), Stock: float32(row.Stock)}
		if row.CostPerUnit != nil {
			c := float32(*row.CostPerUnit)
			item.CostPerUnit = &c
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) AdjustStock(w http.ResponseWriter, r *http.Request, ingredientID openapi_types.UUID) {
	var in StockAdjust
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	n, err := h.inv.AdjustStock(r.Context(), uuid.UUID(ingredientID), float64(in.Delta))
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]float64{"stock": n})
}
