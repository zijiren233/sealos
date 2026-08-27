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
	"errors"
	"testing"

	userv1 "github.com/labring/sealos/controllers/user/api/v1"
	"github.com/labring/sealos/controllers/user/controllers/helper/config"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type namespaceCreateErrorClient struct {
	client.Client
}

func (c namespaceCreateErrorClient) Create(
	ctx context.Context,
	obj client.Object,
	opts ...client.CreateOption,
) error {
	if _, ok := obj.(*corev1.Namespace); ok {
		return errors.New("namespace create failed")
	}
	return c.Client.Create(ctx, obj, opts...)
}

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
	if err := cli.Get(
		context.Background(),
		client.ObjectKeyFromObject(stored),
		projected,
	); err != nil {
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

func TestOwnerAnnotationChangedPredicate(t *testing.T) {
	t.Parallel()

	oldUser := &userv1.User{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{
			userv1.UserAnnotationOwnerKey:   "owner-a",
			userv1.UserAnnotationDisplayKey: "old-display",
		},
	}}
	newUser := oldUser.DeepCopy()
	newUser.Annotations[userv1.UserAnnotationDisplayKey] = "new-display"
	predicate := OwnerAnnotationChangedPredicate{}
	if predicate.Update(event.UpdateEvent{ObjectOld: oldUser, ObjectNew: newUser}) {
		t.Fatal("unrelated annotation change triggered reconciliation")
	}

	newUser.Annotations[userv1.UserAnnotationOwnerKey] = "owner-b"
	if !predicate.Update(event.UpdateEvent{ObjectOld: oldUser, ObjectNew: newUser}) {
		t.Fatal("owner annotation change did not trigger reconciliation")
	}
}

func TestNamespacePodSecurityPredicate(t *testing.T) {
	t.Parallel()

	oldNamespace := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
		Name: "ns-user-a",
		Labels: map[string]string{
			config.PodSecurityLabelPrefix + "enforce": "baseline",
		},
	}}
	newNamespace := oldNamespace.DeepCopy()
	delete(newNamespace.Labels, config.PodSecurityLabelPrefix+"enforce")
	p := NamespacePodSecurityPredicate{}
	if !p.Update(event.UpdateEvent{ObjectOld: oldNamespace, ObjectNew: newNamespace}) {
		t.Fatal("Pod Security label deletion did not trigger reconciliation")
	}

	newNamespace = oldNamespace.DeepCopy()
	newNamespace.Labels["unused.example/label"] = "changed"
	if p.Update(event.UpdateEvent{ObjectOld: oldNamespace, ObjectNew: newNamespace}) {
		t.Fatal("unrelated label change triggered reconciliation")
	}
	if p.Create(event.CreateEvent{Object: oldNamespace}) {
		t.Fatal("namespace create should be handled by user reconciliation")
	}
	if !p.Delete(event.DeleteEvent{Object: oldNamespace}) {
		t.Fatal("managed namespace deletion did not trigger reconciliation")
	}
	if p.Delete(event.DeleteEvent{Object: &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}}) {
		t.Fatal("unmanaged namespace deletion triggered reconciliation")
	}
}

func TestNamespaceToUserRequests(t *testing.T) {
	t.Parallel()
	r := &UserReconciler{}
	requests := r.namespaceToUserRequests(context.Background(), &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-admin"},
	})
	if len(requests) != 1 || requests[0].Name != "admin" {
		t.Fatalf("admin namespace request = %#v", requests)
	}
	if requests := r.namespaceToUserRequests(context.Background(), &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
	}); requests != nil {
		t.Fatalf("unmanaged namespace requests = %#v", requests)
	}
}

func TestAdminClusterRoleBindingPredicate(t *testing.T) {
	t.Parallel()
	p := AdminClusterRoleBindingPredicate{}
	target := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: adminClusterRoleBindingName}}
	other := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "other-binding"}}
	if !p.Create(event.CreateEvent{Object: target}) || !p.Update(event.UpdateEvent{ObjectNew: target}) || !p.Delete(event.DeleteEvent{Object: target}) {
		t.Fatal("admin cluster role binding events were not accepted")
	}
	if p.Create(event.CreateEvent{Object: other}) || p.Update(event.UpdateEvent{ObjectNew: other}) || p.Delete(event.DeleteEvent{Object: other}) {
		t.Fatal("unrelated cluster role binding event was accepted")
	}
}

func TestDesiredNamespaceLabels(t *testing.T) {
	t.Parallel()
	labels := map[string]string{
		config.PodSecurityLabelPrefix + "enforce": "baseline",
		"example.com/keep":                        "value",
	}
	adminLabels := desiredNamespaceLabels("ns-admin", labels, false)
	if adminLabels[config.PodSecurityLabelPrefix+"enforce"] != "baseline" {
		t.Fatal("admin namespace did not receive Pod Security labels when admin privilege is disabled")
	}
	privilegedLabels := desiredNamespaceLabels("ns-admin", adminLabels, true)
	if _, ok := privilegedLabels[config.PodSecurityLabelPrefix+"enforce"]; ok {
		t.Fatal("admin Pod Security labels were retained when admin privilege is enabled")
	}
	if privilegedLabels["example.com/keep"] != "value" {
		t.Fatal("unrelated namespace label was removed")
	}
}

func TestAdminClusterRoleBindingCleanup(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add RBAC scheme: %v", err)
	}
	binding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: adminClusterRoleBindingName}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(binding).Build()
	cleanup := &adminPrivilegeMigration{client: cli}
	if err := cleanup.Start(context.Background()); err != nil {
		t.Fatalf("cleanup binding: %v", err)
	}
	if err := cli.Get(context.Background(), client.ObjectKey{Name: adminClusterRoleBindingName}, &rbacv1.ClusterRoleBinding{}); !apierrors.IsNotFound(err) {
		t.Fatalf("binding still exists, get error = %v", err)
	}
}

func TestAdminPrivilegeMigrationDisablesLegacyBinding(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add RBAC scheme: %v", err)
	}
	if err := userv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add user scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&userv1.User{ObjectMeta: metav1.ObjectMeta{Name: adminUserName}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: adminClusterRoleBindingName}},
	).Build()
	r := &UserReconciler{Client: cli, Scheme: scheme}
	migration := &adminPrivilegeMigration{
		client:     cli,
		reader:     cli,
		reconciler: r,
	}
	if err := migration.Start(context.Background()); err != nil {
		t.Fatalf("disable admin privilege: %v", err)
	}
	if err := cli.Get(context.Background(), client.ObjectKey{Name: adminClusterRoleBindingName}, &rbacv1.ClusterRoleBinding{}); !apierrors.IsNotFound(err) {
		t.Fatalf("legacy binding still exists, get error = %v", err)
	}
	ns := &corev1.Namespace{}
	if err := cli.Get(context.Background(), client.ObjectKey{Name: config.GetUsersNamespace(adminUserName)}, ns); err != nil {
		t.Fatalf("get admin namespace: %v", err)
	}
	if ns.Labels[config.PodSecurityLabelPrefix+"enforce"] != "baseline" {
		t.Fatalf("admin namespace labels = %#v", ns.Labels)
	}
}

func TestAdminPrivilegeMigrationEnablesLegacyBinding(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add RBAC scheme: %v", err)
	}
	if err := userv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add user scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&userv1.User{ObjectMeta: metav1.ObjectMeta{Name: adminUserName}},
	).Build()
	r := &UserReconciler{Client: cli, Scheme: scheme, EnableAdminClusterAdmin: true}
	migration := &adminPrivilegeMigration{
		client:                  cli,
		reader:                  cli,
		reconciler:              r,
		enableAdminClusterAdmin: true,
	}
	if err := migration.Start(context.Background()); err != nil {
		t.Fatalf("enable admin privilege: %v", err)
	}
	if err := cli.Get(context.Background(), client.ObjectKey{Name: adminClusterRoleBindingName}, &rbacv1.ClusterRoleBinding{}); err != nil {
		t.Fatalf("get restored binding: %v", err)
	}
	ns := &corev1.Namespace{}
	if err := cli.Get(context.Background(), client.ObjectKey{Name: config.GetUsersNamespace(adminUserName)}, ns); err != nil {
		t.Fatalf("get admin namespace: %v", err)
	}
	if _, ok := ns.Labels[config.PodSecurityLabelPrefix+"enforce"]; ok {
		t.Fatalf("admin namespace retained Pod Security labels: %#v", ns.Labels)
	}
}

func TestAdminPrivilegeMigrationPropagatesSyncError(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add RBAC scheme: %v", err)
	}
	if err := userv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add user scheme: %v", err)
	}
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&userv1.User{ObjectMeta: metav1.ObjectMeta{Name: adminUserName}},
	).Build()
	failingClient := namespaceCreateErrorClient{Client: baseClient}
	r := &UserReconciler{
		Client:   failingClient,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	migration := &adminPrivilegeMigration{
		client:                  failingClient,
		reader:                  baseClient,
		reconciler:              r,
		enableAdminClusterAdmin: true,
	}
	if err := migration.Start(context.Background()); err == nil {
		t.Fatal("admin privilege migration reported success after namespace sync failed")
	}
}

func TestPatchUserOwnerPreservesUncachedAnnotations(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := userv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add user scheme: %v", err)
	}
	stored := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-a",
			Annotations: map[string]string{
				userv1.UserAnnotationOwnerKey:   "old-owner",
				userv1.UserAnnotationDisplayKey: "display-name",
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stored).Build()

	projected := &userv1.User{}
	if err := cli.Get(
		context.Background(),
		client.ObjectKeyFromObject(stored),
		projected,
	); err != nil {
		t.Fatalf("get user: %v", err)
	}
	projected.Annotations = map[string]string{
		userv1.UserAnnotationOwnerKey: projected.Annotations[userv1.UserAnnotationOwnerKey],
	}

	reconciler := &OperationReqReconciler{Client: cli}
	if err := reconciler.patchUserOwner(context.Background(), projected, "new-owner"); err != nil {
		t.Fatalf("patch user owner: %v", err)
	}

	got := &userv1.User{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(stored), got); err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if got.Annotations[userv1.UserAnnotationOwnerKey] != "new-owner" {
		t.Fatalf("owner = %q, want new-owner", got.Annotations[userv1.UserAnnotationOwnerKey])
	}
	if got.Annotations[userv1.UserAnnotationDisplayKey] != "display-name" {
		t.Fatalf(
			"display annotation = %q, want display-name",
			got.Annotations[userv1.UserAnnotationDisplayKey],
		)
	}
}
