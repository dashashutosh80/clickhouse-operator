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

package v1

import (
	"github.com/altinity/clickhouse-operator/pkg/apis/common/types"
)

const (
	// HookTargetFirstHost runs the action on cluster.FirstHost() only. Default for cluster hooks.
	HookTargetFirstHost = "firstHost"
	// HookTargetAllHosts runs the action on every host in the cluster.
	HookTargetAllHosts = "allHosts"
	// HookTargetAllShards runs the action on the first replica of each shard.
	HookTargetAllShards = "allShards"
)

// HookAction defines one action to execute at a reconcile lifecycle point.
// Exactly one of the action type fields must be specified.
type HookAction struct {
	// SQL executes SQL queries against ClickHouse.
	// +optional
	SQL *SQLHookAction `json:"sql,omitempty" yaml:"sql,omitempty"`
	// Shell executes a command inside the pod. Not yet implemented.
	// +optional
	Shell *ShellHookAction `json:"shell,omitempty" yaml:"shell,omitempty"`
	// HTTP makes an HTTP request to an endpoint. Not yet implemented.
	// +optional
	HTTP *HTTPHookAction `json:"http,omitempty" yaml:"http,omitempty"`
	// Target specifies which host(s) to execute this action on, for cluster-level hooks.
	// Valid values: "firstHost" (default), "allHosts", "allShards".
	// Ignored for host-level hooks — always executes on the host being reconciled.
	// +optional
	Target *types.String `json:"target,omitempty" yaml:"target,omitempty"`
}

// IsEmpty returns true if the action has no recognized type set.
func (a *HookAction) IsEmpty() bool {
	return a == nil || (a.SQL == nil && a.Shell == nil && a.HTTP == nil)
}

// HasSQL returns true if the action is a SQL hook.
func (a *HookAction) HasSQL() bool {
	return a != nil && a.SQL != nil
}

// HasShell returns true if the action is a Shell hook.
func (a *HookAction) HasShell() bool {
	return a != nil && a.Shell != nil
}

// HasHTTP returns true if the action is an HTTP hook.
func (a *HookAction) HasHTTP() bool {
	return a != nil && a.HTTP != nil
}

// SQLHookAction executes SQL queries against ClickHouse.
type SQLHookAction struct {
	// Queries is a list of SQL statements to execute sequentially.
	Queries []string `json:"queries,omitempty" yaml:"queries,omitempty"`
}

// ShellHookAction executes a command inside a pod container.
// Reserved for future implementation.
type ShellHookAction struct {
	// Command is the command and its arguments.
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
	// Container specifies the container to run the command in. Defaults to the ClickHouse container.
	// +optional
	Container *types.String `json:"container,omitempty" yaml:"container,omitempty"`
}

// HTTPHookAction makes an HTTP request to an endpoint.
// Reserved for future implementation.
type HTTPHookAction struct {
	// URL is the target endpoint.
	URL *types.String `json:"url,omitempty" yaml:"url,omitempty"`
	// Method is the HTTP method. Defaults to GET.
	// +optional
	Method *types.String `json:"method,omitempty" yaml:"method,omitempty"`
}

// ReconcileHooks defines pre/post actions for a reconcile lifecycle scope.
type ReconcileHooks struct {
	// Pre is a list of actions to execute before the reconcile step.
	// +optional
	Pre []*HookAction `json:"pre,omitempty" yaml:"pre,omitempty"`
	// Post is a list of actions to execute after the reconcile step.
	// +optional
	Post []*HookAction `json:"post,omitempty" yaml:"post,omitempty"`
}

// GetPre returns pre-hooks or nil.
func (h *ReconcileHooks) GetPre() []*HookAction {
	if h == nil {
		return nil
	}
	return h.Pre
}

// GetPost returns post-hooks or nil.
func (h *ReconcileHooks) GetPost() []*HookAction {
	if h == nil {
		return nil
	}
	return h.Post
}

// IsEmpty returns true if there are no pre or post hooks.
func (h *ReconcileHooks) IsEmpty() bool {
	return len(h.GetPre()) == 0 && len(h.GetPost()) == 0
}

// MergeFrom merges hooks from a parent scope.
// Actions from parent are appended after the receiver's actions (parent runs first, then child).
func (h *ReconcileHooks) MergeFrom(from *ReconcileHooks) *ReconcileHooks {
	if from == nil {
		return h
	}
	if h == nil {
		return from.DeepCopy()
	}
	h.Pre = mergeHookActions(h.Pre, from.Pre)
	h.Post = mergeHookActions(h.Post, from.Post)
	return h
}

// mergeHookActions appends actions from parent that are not already present in child.
func mergeHookActions(child, parent []*HookAction) []*HookAction {
	for _, p := range parent {
		if p == nil {
			continue
		}
		child = append(child, p.DeepCopy())
	}
	return child
}

// MergeFrom merges a HookAction from a parent, filling empty fields.
func (a *HookAction) MergeFrom(from *HookAction) *HookAction {
	if from == nil {
		return a
	}
	if a == nil {
		return from.DeepCopy()
	}
	if a.SQL == nil {
		a.SQL = from.SQL
	} else if from.SQL != nil {
		a.SQL.MergeFrom(from.SQL)
	}
	if a.Shell == nil {
		a.Shell = from.Shell
	} else if from.Shell != nil {
		a.Shell.MergeFrom(from.Shell)
	}
	if a.HTTP == nil {
		a.HTTP = from.HTTP
	} else if from.HTTP != nil {
		a.HTTP.MergeFrom(from.HTTP)
	}
	a.Target = a.Target.MergeFrom(from.Target)
	return a
}

// MergeFrom merges SQL hook from parent, appending queries.
func (s *SQLHookAction) MergeFrom(from *SQLHookAction) {
	if from == nil || s == nil {
		return
	}
	s.Queries = append(s.Queries, from.Queries...)
}

// MergeFrom merges Shell hook from parent, appending commands and filling empty container.
func (sh *ShellHookAction) MergeFrom(from *ShellHookAction) {
	if from == nil || sh == nil {
		return
	}
	sh.Command = append(sh.Command, from.Command...)
	sh.Container = sh.Container.MergeFrom(from.Container)
}

// MergeFrom merges HTTP hook from parent, filling empty fields.
func (h *HTTPHookAction) MergeFrom(from *HTTPHookAction) {
	if from == nil || h == nil {
		return
	}
	h.URL = h.URL.MergeFrom(from.URL)
	h.Method = h.Method.MergeFrom(from.Method)
}
