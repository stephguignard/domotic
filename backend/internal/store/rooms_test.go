package store

import (
	"context"
	"slices"
	"testing"
)

func TestRoomOrderRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if got, err := s.RoomOrder(ctx); err != nil || len(got) != 0 || got == nil {
		t.Fatalf("ordre initial = %#v, %v ; attendu une liste vide non nil", got, err)
	}

	want := []string{"Salon", "Cuisine", "Lily"}
	if err := s.SetRoomOrder(ctx, want); err != nil {
		t.Fatalf("SetRoomOrder: %v", err)
	}
	if got, _ := s.RoomOrder(ctx); !slices.Equal(got, want) {
		t.Errorf("ordre = %v, attendu %v", got, want)
	}

	// Un nouvel ordre remplace le précédent en entier.
	if err := s.SetRoomOrder(ctx, []string{"Lily"}); err != nil {
		t.Fatalf("SetRoomOrder: %v", err)
	}
	if got, _ := s.RoomOrder(ctx); !slices.Equal(got, []string{"Lily"}) {
		t.Errorf("après remplacement = %v", got)
	}

	// Un doublon est refusé, sans toucher à l'ordre en place.
	if err := s.SetRoomOrder(ctx, []string{"Salon", "Salon"}); err == nil {
		t.Error("attendu un refus pour une pièce en double")
	}
	if got, _ := s.RoomOrder(ctx); !slices.Equal(got, []string{"Lily"}) {
		t.Errorf("ordre modifié malgré l'échec : %v", got)
	}

	if err := s.SetRoomOrder(ctx, nil); err != nil {
		t.Fatalf("SetRoomOrder vide: %v", err)
	}
	if got, _ := s.RoomOrder(ctx); len(got) != 0 {
		t.Errorf("après remise à zéro = %v", got)
	}
}
