// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build kubeapiserver

package executors

import (
	"context"
	"encoding/json"
	"fmt"

	kubeactions "github.com/DataDog/agent-payload/v5/kubeactions"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// PatchDeploymentExecutor executes patch deployment actions
type PatchDeploymentExecutor struct {
	clientset kubernetes.Interface
}

// NewPatchDeploymentExecutor creates a new PatchDeploymentExecutor
func NewPatchDeploymentExecutor(clientset kubernetes.Interface) *PatchDeploymentExecutor {
	return &PatchDeploymentExecutor{
		clientset: clientset,
	}
}

// Execute applies a strategic merge patch to a deployment
func (e *PatchDeploymentExecutor) Execute(ctx context.Context, action *kubeactions.KubeAction) ExecutionResult {
	resource := action.Resource
	namespace := resource.Namespace
	name := resource.Name

	// Validate namespace is provided
	if namespace == "" {
		return ExecutionResult{
			Status:  "failed",
			Message: "namespace is required for deployment patch",
		}
	}

	// Validate resource_id is provided for UID safety check
	resourceID := resource.ResourceId
	if resourceID == "" {
		return ExecutionResult{
			Status:  "failed",
			Message: "resource_id is required for deployment patch (used for UID safety check)",
		}
	}

	// Get patch params
	patchParams := action.GetPatchDeployment()
	if patchParams == nil || len(patchParams.GetPatch()) == 0 {
		return ExecutionResult{
			Status:  "failed",
			Message: "patch is required for patch_deployment action",
		}
	}

	patchBytes := patchParams.GetPatch()

	// Validate the patch is valid JSON
	if !json.Valid(patchBytes) {
		return ExecutionResult{
			Status:  "failed",
			Message: "patch must be valid JSON",
		}
	}

	log.Infof("Patching deployment %s/%s (uid=%s) with patch: %s", namespace, name, resourceID, string(patchBytes))

	// Get the deployment first to verify UID matches resource_id
	deployment, err := e.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get deployment %s/%s: %v", namespace, name, err)
		return ExecutionResult{
			Status:  "failed",
			Message: fmt.Sprintf("failed to get deployment: %v", err),
		}
	}

	if string(deployment.UID) != resourceID {
		log.Errorf("Deployment %s/%s UID mismatch: expected %s, got %s - deployment may have been replaced", namespace, name, resourceID, deployment.UID)
		return ExecutionResult{
			Status:  "failed",
			Message: fmt.Sprintf("deployment UID mismatch: expected %s, got %s - deployment may have been replaced since action was created", resourceID, deployment.UID),
		}
	}

	// Apply the strategic merge patch
	_, err = e.clientset.AppsV1().Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		log.Errorf("Failed to patch deployment %s/%s: %v", namespace, name, err)
		return ExecutionResult{
			Status:  "failed",
			Message: fmt.Sprintf("failed to patch deployment: %v", err),
		}
	}

	log.Infof("Successfully patched deployment %s/%s", namespace, name)
	return ExecutionResult{
		Status:  "success",
		Message: fmt.Sprintf("deployment %s/%s patched", namespace, name),
	}
}
