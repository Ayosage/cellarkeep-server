package store_test

import (
	"context"
	"testing"

	"github.com/ayosage/cellarkeep-server/internal/store/gen"
	"github.com/ayosage/cellarkeep-server/internal/store/testdb"
)

func TestMigrationsCreateVessels(t *testing.T) {
	s := testdb.Open(t)
	q := testdb.Tx(t, s)
	v, err := q.CreateVessel(context.Background(), gen.CreateVesselParams{
		Name: "Bucket #1", Kind: gen.VesselKindBucket, CapacityL: 23,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.GetVessel(context.Background(), v.ID)
	if err != nil || got.Name != "Bucket #1" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
