// Copyright 2026 labring.
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

package controllers

import (
	"context"
	"testing"

	userv1 "github.com/labring/sealos/controllers/user/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateStatusPreservesUncachedKubeConfig(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := userv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add user scheme: %v", err)
	}
	stored := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user-a"},
		Status: userv1.UserStatus{
			Phase:      userv1.UserActive,
			KubeConfig: "existing-kubeconfig",
		},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&userv1.User{}).
		WithObjects(stored).
		Build()

	projected := &userv1.User{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(stored), projected); err != nil {
		t.Fatalf("get user: %v", err)
	}
	projected.Status.KubeConfig = ""
	originalStatus := projected.Status.DeepCopy()
	projected.Status.ObservedGeneration = 1

	reconciler := &UserReconciler{Client: cli}
	if err := reconciler.updateStatus(context.Background(), projected, originalStatus); err != nil {
		t.Fatalf("patch status: %v", err)
	}

	got := &userv1.User{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(stored), got); err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if got.Status.KubeConfig != stored.Status.KubeConfig {
		t.Fatalf("kubeconfig = %q, want %q", got.Status.KubeConfig, stored.Status.KubeConfig)
	}
	if got.Status.ObservedGeneration != 1 {
		t.Fatalf("observed generation = %d, want 1", got.Status.ObservedGeneration)
	}
}
