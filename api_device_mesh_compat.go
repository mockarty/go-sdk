// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import "context"

// SubmitCommand preserves the published command submission API.
// Deprecated: use Submit. Both methods preserve the caller's retry identity.
func (a *DeviceMeshAPI) SubmitCommand(ctx context.Context, namespace string, in MeshSubmitInput) (MeshCommand, bool, error) {
	return a.Submit(ctx, namespace, in)
}

// ReportCommand preserves the published executing-device report API.
// Deprecated: use Report. The same original lease and report are used.
func (a *DeviceMeshAPI) ReportCommand(ctx context.Context, namespace, deviceID, commandID string, report MeshCommandReport) (MeshCommand, error) {
	return a.Report(ctx, namespace, deviceID, commandID, report)
}
