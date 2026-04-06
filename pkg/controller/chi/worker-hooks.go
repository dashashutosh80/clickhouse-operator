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

package chi

import (
	"context"
	"fmt"

	api "github.com/altinity/clickhouse-operator/pkg/apis/clickhouse.altinity.com/v1"
	a "github.com/altinity/clickhouse-operator/pkg/controller/common/announcer"
)

// clusterHasRunningHosts checks whether the cluster has hosts from a previous reconcile.
// On first CHI creation, hosts have no ancestors — hooks would fail since pods don't exist.
func clusterHasRunningHosts(cluster *api.Cluster) bool {
	firstHost := cluster.FirstHost()
	if firstHost == nil {
		return false
	}
	return firstHost.HasAncestor()
}

// runClusterPreHooks executes cluster-level pre-reconcile hooks.
// Returns error on failure — caller should abort the cluster reconcile.
// Skipped on first cluster creation (no live hosts to execute SQL on).
func (w *worker) runClusterPreHooks(ctx context.Context, cluster *api.Cluster) error {
	hooks := cluster.GetReconcile().GetHooks()
	if hooks.IsEmpty() {
		return nil
	}
	if !clusterHasRunningHosts(cluster) {
		w.a.V(1).M(cluster).F().Info("Skipping cluster pre-hooks: no live hosts yet (first creation)")
		return nil
	}
	return w.runClusterHookActions(ctx, hooks.GetPre(), cluster)
}

// runClusterPostHooks executes cluster-level post-reconcile hooks.
// Errors are logged but do not fail the reconcile.
func (w *worker) runClusterPostHooks(ctx context.Context, cluster *api.Cluster) {
	hooks := cluster.GetReconcile().GetHooks()
	if hooks.IsEmpty() {
		return
	}
	if err := w.runClusterHookActions(ctx, hooks.GetPost(), cluster); err != nil {
		w.a.V(1).M(cluster).F().
			WithEvent(cluster.GetRuntime().GetCR(), a.EventActionReconcile, a.EventReasonReconcileFailed).
			Warning("Cluster post-hook failed: %v", err)
	}
}

// runHostPreHooks executes host-level pre-reconcile hooks.
// Returns error on failure — caller should abort the host reconcile.
// Skipped for hosts that have no ancestor (first creation).
func (w *worker) runHostPreHooks(ctx context.Context, host *api.Host) error {
	hooks := host.GetCluster().GetReconcile().Host.GetHooks()
	if hooks.IsEmpty() {
		return nil
	}
	if !host.HasAncestor() {
		w.a.V(1).M(host).F().Info("Skipping host pre-hooks: host has no ancestor (first creation)")
		return nil
	}
	return w.runHostHookActions(ctx, hooks.GetPre(), host)
}

// runHostPostHooks executes host-level post-reconcile hooks.
// Errors are logged but do not fail the reconcile.
func (w *worker) runHostPostHooks(ctx context.Context, host *api.Host) {
	hooks := host.GetCluster().GetReconcile().Host.GetHooks()
	if hooks.IsEmpty() {
		return
	}
	if err := w.runHostHookActions(ctx, hooks.GetPost(), host); err != nil {
		w.a.V(1).M(host).F().
			WithEvent(host.GetCR(), a.EventActionReconcile, a.EventReasonReconcileFailed).
			Warning("Host post-hook failed: %v", err)
	}
}

// runClusterHookActions iterates over cluster-level hook actions and executes them sequentially.
func (w *worker) runClusterHookActions(ctx context.Context, actions []*api.HookAction, cluster *api.Cluster) error {
	for _, action := range actions {
		switch {
		case action.HasSQL():
			if err := w.runClusterSQLHookAction(ctx, action, cluster); err != nil {
				return err
			}
		case action.HasShell():
			return fmt.Errorf("shell hooks not yet implemented")
		case action.HasHTTP():
			return fmt.Errorf("http hooks not yet implemented")
		default:
			w.a.V(1).F().Info("Empty action specified")
		}
	}
	return nil
}

// runHostHookActions iterates over host-level hook actions and executes them sequentially.
func (w *worker) runHostHookActions(ctx context.Context, actions []*api.HookAction, host *api.Host) error {
	for _, action := range actions {
		switch {
		case action.HasSQL():
			if err := w.runHostSQLHookAction(ctx, action, host); err != nil {
				return err
			}
		case action.HasShell():
			return fmt.Errorf("shell hooks not yet implemented")
		case action.HasHTTP():
			return fmt.Errorf("http hooks not yet implemented")
		default:
			w.a.V(1).F().Info("Empty action specified")
		}
	}
	return nil
}

// runHostSQLHookAction executes a SQL hook action on a specific host.
func (w *worker) runHostSQLHookAction(ctx context.Context, action *api.HookAction, host *api.Host) error {
	if !action.HasSQL() {
		// Sanity check
		return nil
	}

	sql := action.SQL
	if len(sql.Queries) == 0 {
		// Sanity check
		return nil
	}

	w.a.V(1).M(host).F().Info("Running SQL host hook on %s: %v", host.GetName(), sql.Queries)
	return w.ensureClusterSchemer(host).ExecHost(ctx, host, sql.Queries)
}

// runClusterSQLHookAction executes a SQL hook action at cluster scope, respecting action.Target.
func (w *worker) runClusterSQLHookAction(ctx context.Context, action *api.HookAction, cluster *api.Cluster) error {
	if !action.HasSQL() {
		// Sanity check
		return nil
	}

	sql := action.SQL
	if len(sql.Queries) == 0 {
		// Sanity check
		return nil
	}

	switch action.Target.Value() {
	case api.HookTargetAllHosts:
		w.a.V(1).M(cluster).F().Info("Running SQL cluster hook on all hosts: %v", sql.Queries)
		firstHost := cluster.FirstHost()
		if firstHost == nil {
			return fmt.Errorf("cluster %s has no hosts for hook execution", cluster.GetName())
		}
		return w.ensureClusterSchemer(firstHost).ExecCluster(ctx, cluster, sql.Queries)

	case api.HookTargetAllShards:
		w.a.V(1).M(cluster).F().Info("Running SQL cluster hook on all shards (first replica each): %v", sql.Queries)
		var firstErr error
		cluster.WalkShards(func(_ int, shard api.IShard) error {
			chiShard, ok := shard.(*api.ChiShard)
			if !ok {
				return nil
			}
			h := chiShard.FirstHost()
			if h == nil {
				return nil
			}
			if err := w.ensureClusterSchemer(h).ExecHost(ctx, h, sql.Queries); err != nil && firstErr == nil {
				firstErr = err
			}
			return nil
		})
		return firstErr

	default: // HookTargetFirstHost or empty
		firstHost := cluster.FirstHost()
		if firstHost == nil {
			return fmt.Errorf("cluster %s has no hosts for hook execution", cluster.GetName())
		}
		w.a.V(1).M(cluster).F().Info("Running SQL cluster hook on first host %s: %v", firstHost.GetName(), sql.Queries)
		return w.ensureClusterSchemer(firstHost).ExecHost(ctx, firstHost, sql.Queries)
	}
}
