// Copyright 2019 Altinity Ltd and/or its affiliates. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package creator

import (
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	api "github.com/altinity/clickhouse-operator/pkg/apis/clickhouse.altinity.com/v1"
	"github.com/altinity/clickhouse-operator/pkg/interfaces"
)

type ProbeManager struct {
}

func NewProbeManager() *ProbeManager {
	return &ProbeManager{}
}

func (m *ProbeManager) CreateProbe(what interfaces.ProbeType, host *api.Host) *core.Probe {
	switch what {
	case interfaces.ProbeDefaultStartup:
		return m.createDefaultStartupProbe(host)
	case interfaces.ProbeDefaultLiveness:
		return m.createDefaultLivenessProbe(host)
	case interfaces.ProbeDefaultReadiness:
		return m.createDefaultReadinessProbe(host)
	}
	panic("unknown probe type")
}

// createDefaultStartupProbe returns default startup probe.
// Uses pgrep to check that the clickhouse-keeper process is running.
// This is intentionally quorum-independent: the probe succeeds as soon as the
// process starts, allowing the operator to proceed to the next host without
// waiting for Raft quorum. FailureThreshold is generous to handle slow starts.
func (m *ProbeManager) createDefaultStartupProbe(_ *api.Host) *core.Probe {
	return &core.Probe{
		ProbeHandler: core.ProbeHandler{
			Exec: &core.ExecAction{
				Command: []string{"pgrep", "clickhouse-keeper"},
			},
		},
		InitialDelaySeconds: 1,
		PeriodSeconds:       3,
		FailureThreshold:    60,
	}
}

// createDefaultLivenessProbe returns default liveness probe.
// Uses pgrep to detect a crashed or hung clickhouse-keeper process.
// Readiness (Raft quorum) is covered by the separate readiness probe.
func (m *ProbeManager) createDefaultLivenessProbe(_ *api.Host) *core.Probe {
	return &core.Probe{
		ProbeHandler: core.ProbeHandler{
			Exec: &core.ExecAction{
				Command: []string{"pgrep", "clickhouse-keeper"},
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		FailureThreshold:    12,
	}
}

// createDefaultReadinessProbe returns default readiness probe.
// Checks the /ready HTTP endpoint which reflects Raft quorum status.
func (m *ProbeManager) createDefaultReadinessProbe(_ *api.Host) *core.Probe {
	return &core.Probe{
		ProbeHandler: core.ProbeHandler{
			HTTPGet: &core.HTTPGetAction{
				Path: "/ready",
				Port: intstr.Parse("9182"),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		FailureThreshold:    12,
	}
}
