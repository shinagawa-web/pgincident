package core

import "testing"

func TestConnectionsStatus(t *testing.T) {
	cases := []struct {
		active, max int
		want        HealthStatus
	}{
		{0, 0, StatusNormal},      // max=0 guard
		{70, 100, StatusNormal},   // 70% — OK
		{85, 100, StatusWarning},  // 85% — warn
		{92, 100, StatusCritical}, // 92% — crit
	}
	for _, c := range cases {
		s := DBStats{ConnectionsActive: c.active, ConnectionsMax: c.max}
		if got := s.ConnectionsStatus(); got != c.want {
			t.Errorf("ConnectionsStatus(%d/%d) = %v, want %v", c.active, c.max, got, c.want)
		}
	}
}

func TestCacheHitStatus(t *testing.T) {
	cases := []struct {
		ratio float64
		want  HealthStatus
	}{
		{0.999, StatusNormal},
		{0.990, StatusNormal},   // exactly at warn threshold — still normal
		{0.985, StatusWarning},  // between warn and crit
		{0.949, StatusCritical}, // below crit
	}
	for _, c := range cases {
		s := DBStats{CacheHitRatio: c.ratio}
		if got := s.CacheHitStatus(); got != c.want {
			t.Errorf("CacheHitStatus(%.3f) = %v, want %v", c.ratio, got, c.want)
		}
	}
}

func TestCheckpointStatus(t *testing.T) {
	cases := []struct {
		req  int64
		want HealthStatus
	}{
		{0, StatusNormal},
		{1, StatusWarning},
		{100, StatusWarning},
	}
	for _, c := range cases {
		s := DBStats{CheckpointReq: c.req}
		if got := s.CheckpointStatus(); got != c.want {
			t.Errorf("CheckpointStatus(%d) = %v, want %v", c.req, got, c.want)
		}
	}
}

func TestReplicationLagStatus(t *testing.T) {
	cases := []struct {
		secs float64
		want HealthStatus
	}{
		{0, StatusNormal},
		{4.9, StatusNormal},
		{5.1, StatusWarning},
		{30.1, StatusCritical},
	}
	for _, c := range cases {
		s := DBStats{ReplicationLagSecs: c.secs}
		if got := s.ReplicationLagStatus(); got != c.want {
			t.Errorf("ReplicationLagStatus(%.1f) = %v, want %v", c.secs, got, c.want)
		}
	}
}

func TestAutovacuumStatus(t *testing.T) {
	cases := []struct {
		workers int
		want    HealthStatus
	}{
		{0, StatusNormal},
		{3, StatusNormal}, // exactly at threshold — still normal
		{4, StatusWarning},
		{5, StatusWarning}, // exactly at crit threshold — still warn
		{6, StatusCritical},
	}
	for _, c := range cases {
		s := DBStats{AutovacuumWorkers: c.workers}
		if got := s.AutovacuumStatus(); got != c.want {
			t.Errorf("AutovacuumStatus(%d) = %v, want %v", c.workers, got, c.want)
		}
	}
}
