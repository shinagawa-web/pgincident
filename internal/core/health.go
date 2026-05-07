package core

// HealthStatus represents the severity level of a metric.
type HealthStatus int

const (
	StatusNormal HealthStatus = iota
	StatusWarning
	StatusCritical
)

const (
	connWarnPct     = 0.80
	connCritPct     = 0.90
	cacheWarnRatio  = 0.99
	cacheCritRatio  = 0.95
	replWarnSecs    = 5.0
	replCritSecs    = 30.0
	avacWarnWorkers = 3
	avacCritWorkers = 5
)

func (s DBStats) ConnectionsStatus() HealthStatus {
	if s.ConnectionsMax == 0 {
		return StatusNormal
	}
	pct := float64(s.ConnectionsActive) / float64(s.ConnectionsMax)
	switch {
	case pct > connCritPct:
		return StatusCritical
	case pct > connWarnPct:
		return StatusWarning
	default:
		return StatusNormal
	}
}

func (s DBStats) CacheHitStatus() HealthStatus {
	switch {
	case s.CacheHitRatio < cacheCritRatio:
		return StatusCritical
	case s.CacheHitRatio < cacheWarnRatio:
		return StatusWarning
	default:
		return StatusNormal
	}
}

func (s DBStats) CheckpointStatus() HealthStatus {
	if s.CheckpointReq > 0 {
		return StatusWarning
	}
	return StatusNormal
}

func (s DBStats) ReplicationLagStatus() HealthStatus {
	switch {
	case s.ReplicationLagSecs > replCritSecs:
		return StatusCritical
	case s.ReplicationLagSecs > replWarnSecs:
		return StatusWarning
	default:
		return StatusNormal
	}
}

func (s DBStats) AutovacuumStatus() HealthStatus {
	switch {
	case s.AutovacuumWorkers > avacCritWorkers:
		return StatusCritical
	case s.AutovacuumWorkers > avacWarnWorkers:
		return StatusWarning
	default:
		return StatusNormal
	}
}
