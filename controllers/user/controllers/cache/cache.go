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

package cache

import (
	"time"

	licensev1 "github.com/labring/sealos/controllers/license/api/v1"
	userv1 "github.com/labring/sealos/controllers/user/api/v1"
	"github.com/labring/sealos/controllers/user/controllers/helper/config"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Options keeps only explicitly registered informer data in memory. Large
// fields are removed when controllers only need a smaller object projection.
func Options(syncPeriod *time.Duration) ctrlcache.Options {
	return ctrlcache.Options{
		SyncPeriod:                  syncPeriod,
		ReaderFailOnMissingInformer: true,
		DefaultTransform:            ctrlcache.TransformStripManagedFields(),
		ByObject: map[client.Object]ctrlcache.ByObject{
			&userv1.User{}: {
				Transform: transformUser,
			},
			&corev1.Secret{}: {
				Namespaces: map[string]ctrlcache.Config{
					config.GetUserSystemNamespace(): {},
				},
				Transform: transformSecret,
			},
			&corev1.ServiceAccount{}: {
				Namespaces: map[string]ctrlcache.Config{
					config.GetUserSystemNamespace(): {},
				},
			},
			&userv1.Operationrequest{}: {
				Namespaces: map[string]ctrlcache.Config{
					config.GetUserSystemNamespace(): {},
				},
			},
		},
	}
}

// UncachedObjects returns objects whose reads require complete, current API data.
func UncachedObjects() []client.Object {
	return []client.Object{
		&licensev1.License{},
		&userv1.DeleteRequest{},
		&userv1.Operationrequest{},
		&corev1.Namespace{},
		&corev1.Secret{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRoleBinding{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
	}
}

func transformUser(obj any) (any, error) {
	user, ok := obj.(*userv1.User)
	if !ok {
		return obj, nil
	}

	projected := user.DeepCopy()
	projected.ManagedFields = nil
	projected.Status.KubeConfig = ""
	return projected, nil
}

func transformSecret(obj any) (any, error) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return obj, nil
	}

	metadata := metav1.ObjectMeta{
		Name:              secret.Name,
		Namespace:         secret.Namespace,
		UID:               secret.UID,
		ResourceVersion:   secret.ResourceVersion,
		CreationTimestamp: secret.CreationTimestamp,
	}
	if secret.DeletionTimestamp != nil {
		metadata.DeletionTimestamp = secret.DeletionTimestamp.DeepCopy()
	}
	if serviceAccountName, ok := secret.Annotations[corev1.ServiceAccountNameKey]; ok {
		metadata.Annotations = map[string]string{
			corev1.ServiceAccountNameKey: serviceAccountName,
		}
	}
	return &corev1.Secret{TypeMeta: secret.TypeMeta, ObjectMeta: metadata}, nil
}
