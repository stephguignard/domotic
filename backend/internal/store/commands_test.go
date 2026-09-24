package store

import (
	"context"
	"testing"
	"time"
)

func TestCommandLogRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Second)

	entries := []CommandLogEntry{
		{DeviceID: "l-1", DeviceName: "Plafonnier", DeviceKind: "light", Source: "hue",
			Command: "setBrightness", Parameters: []any{40.0}, Success: true, CreatedAt: base},
		{DeviceID: "sw-1", DeviceName: "Eau chaude", DeviceKind: "switch", Source: "shelly",
			Command: "off", Success: false, Error: "module injoignable", CreatedAt: base},
		{DeviceID: "l-1", DeviceName: "Plafonnier", DeviceKind: "light", Source: "hue",
			Command: "setColor", Parameters: []any{"#ff0000"}, Success: true, CreatedAt: base.Add(time.Minute)},
	}
	for _, e := range entries {
		if err := s.RecordCommand(ctx, e); err != nil {
			t.Fatalf("RecordCommand: %v", err)
		}
	}

	all, err := s.ListCommands(ctx, CommandLogFilter{})
	if err != nil {
		t.Fatalf("ListCommands: %v", err)
	}
	// Du plus récent au plus ancien ; à seconde égale, la dernière insérée d'abord.
	var order []string
	for _, e := range all {
		order = append(order, e.Command)
	}
	if got := len(order); got != 3 || order[0] != "setColor" || order[1] != "off" || order[2] != "setBrightness" {
		t.Fatalf("ordre = %v", order)
	}

	off := all[1]
	if off.Success || off.Error != "module injoignable" || len(off.Parameters) != 0 || off.Parameters == nil {
		t.Errorf("échec mal restitué: %+v", off)
	}
	if p := all[2].Parameters; len(p) != 1 || p[0] != 40.0 {
		t.Errorf("paramètres = %v", p)
	}

	mine, err := s.ListCommands(ctx, CommandLogFilter{DeviceID: "l-1"})
	if err != nil {
		t.Fatalf("ListCommands filtré: %v", err)
	}
	if len(mine) != 2 {
		t.Errorf("attendu 2 actions sur l-1, obtenu %d", len(mine))
	}
}

func TestCommandLogSurvivesDeviceRemoval(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.UpsertDevices(ctx, []Device{testDevice("io://1234/1")}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}
	if err := s.RecordCommand(ctx, CommandLogEntry{DeviceID: "io://1234/1", DeviceName: "Volet salon",
		DeviceKind: "shutter", Source: "tahoma", Command: "open", Success: true, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("RecordCommand: %v", err)
	}
	if _, err := s.DB().ExecContext(ctx, `DELETE FROM device`); err != nil {
		t.Fatalf("suppression: %v", err)
	}

	entries, err := s.ListCommands(ctx, CommandLogFilter{})
	if err != nil || len(entries) != 1 || entries[0].DeviceName != "Volet salon" {
		t.Errorf("historique perdu avec l'équipement: %v, %v", entries, err)
	}
}

func TestPurgeCommandsBefore(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Now().UTC()

	for _, at := range []time.Time{now.AddDate(-2, 0, 0), now} {
		if err := s.RecordCommand(ctx, CommandLogEntry{DeviceID: "x", DeviceName: "x", DeviceKind: "light",
			Source: "hue", Command: "on", Success: true, CreatedAt: at}); err != nil {
			t.Fatalf("RecordCommand: %v", err)
		}
	}
	n, err := s.PurgeCommandsBefore(ctx, now.AddDate(-1, 0, 0))
	if err != nil || n != 1 {
		t.Errorf("purge = %d, %v ; attendu 1 ligne", n, err)
	}
}
