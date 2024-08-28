/*
Copyright 2022 labring.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	utilcontroller "github.com/labring/operator-sdk/controller"
	userv1 "github.com/labring/sealos/controllers/user/api/v1"
	"github.com/labring/sealos/controllers/user/controllers/helper"
	"github.com/labring/sealos/controllers/user/controllers/helper/config"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kubecontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// UserNamespaceReconciler reconciles a UserNamespace object
type UserNamespaceReconciler struct {
	Logger   logr.Logger
	Recorder record.EventRecorder
	cache    cache.Cache
	*runtime.Scheme
	client.Client
	finalizer          *utilcontroller.Finalizer
	minRequeueDuration time.Duration
	maxRequeueDuration time.Duration
}

//+kubebuilder:rbac:groups=user.sealos.io,resources=usernamespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=user.sealos.io,resources=usernamespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=user.sealos.io,resources=usernamespaces/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the UserNamespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *UserNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger.V(1).Info("start reconcile for user namespaces")
	userNamespace := &userv1.UserNamespace{}
	if err := r.Get(ctx, req.NamespacedName, userNamespace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ok, err := r.finalizer.RemoveFinalizer(ctx, userNamespace, func(ctx context.Context, obj client.Object) error {
		ns := &v1.Namespace{}
		ns.Name = config.GetUserNamespace(userNamespace.Name)
		return client.IgnoreNotFound(r.Delete(ctx, ns))
	}); ok {
		return ctrl.Result{}, err
	}

	if ok, err := r.finalizer.AddFinalizer(ctx, userNamespace); ok {
		if err != nil {
			return ctrl.Result{}, err
		}
		return r.reconcile(ctx, userNamespace)
	}
	return ctrl.Result{}, errors.New("reconcile error from Finalizer")
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserNamespaceReconciler) SetupWithManager(mgr ctrl.Manager, opts utilcontroller.RateLimiterOptions,
	minRequeueDuration time.Duration, maxRequeueDuration time.Duration) error {
	const controllerName = "usernamespace_controller"
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	r.Logger = ctrl.Log.WithName(controllerName)
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(controllerName)
	}
	if r.finalizer == nil {
		r.finalizer = utilcontroller.NewFinalizer(r.Client, "sealos.io/usernamespace.finalizers")
	}
	r.Scheme = mgr.GetScheme()
	r.cache = mgr.GetCache()
	r.Logger.V(1).Info("init reconcile controller user namespace")
	r.minRequeueDuration = minRequeueDuration
	r.maxRequeueDuration = maxRequeueDuration

	ownerEventHandler := handler.EnqueueRequestForOwner(r.Scheme, r.Client.RESTMapper(), &userv1.UserNamespace{}, handler.OnlyControllerOwner())

	return ctrl.NewControllerManagedBy(mgr).
		For(&userv1.UserNamespace{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}))).
		Watches(&v1.Namespace{}, ownerEventHandler).
		WithOptions(kubecontroller.Options{
			MaxConcurrentReconciles: utilcontroller.GetConcurrent(opts),
			RateLimiter:             utilcontroller.GetRateLimiter(opts),
		}).
		Complete(r)
}

func (r *UserNamespaceReconciler) reconcile(ctx context.Context, obj client.Object) (ctrl.Result, error) {
	r.Logger.V(1).Info("update reconcile controller user namespace", "request", client.ObjectKeyFromObject(obj))
	startTime := time.Now()

	userNamespace, ok := obj.(*userv1.UserNamespace)
	if !ok {
		return ctrl.Result{}, errors.New("obj convert user namespace is error")
	}

	defer func() {
		r.Logger.V(1).Info("finished reconcile", "user namespace info", userNamespace.Name, "create time", userNamespace.CreationTimestamp, "reconcile cost time", time.Since(startTime))
	}()

	pipelines := []func(ctx context.Context, userNamespace *userv1.UserNamespace) context.Context{
		r.initStatus,
		r.syncNamespace,
		r.syncRole,
		r.syncRoleBinding,
		r.syncFinalStatus,
	}

	for _, fn := range pipelines {
		ctx = fn(ctx, userNamespace)
	}

	err := r.updateStatus(ctx, client.ObjectKeyFromObject(obj), userNamespace.Status.DeepCopy())
	if err != nil {
		r.Recorder.Eventf(userNamespace, v1.EventTypeWarning, "SyncStatus", "Sync status %s is error: %v", userNamespace.Name, err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: RandTimeDurationBetween(r.minRequeueDuration, r.maxRequeueDuration)}, nil
}

func (r *UserNamespaceReconciler) initStatus(ctx context.Context, userNamespace *userv1.UserNamespace) context.Context {
	var initializedCondition = userv1.Condition{
		Type:               userv1.Initialized,
		Status:             v1.ConditionTrue,
		Reason:             string(userv1.Initialized),
		Message:            "user namespace has been initialized",
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
	}
	userNamespace.Status.Phase = userv1.UserNamespacePending
	if !helper.IsConditionTrue(userNamespace.Status.Conditions, initializedCondition) {
		userNamespace.Status.Conditions = helper.UpdateCondition(userNamespace.Status.Conditions, initializedCondition)
	}
	return ctx
}

func (r *UserNamespaceReconciler) syncNamespace(ctx context.Context, userNamespace *userv1.UserNamespace) context.Context {
	namespaceConditionType := userv1.ConditionType("NamespaceSyncReady")
	nsCondition := &userv1.Condition{
		Type:               namespaceConditionType,
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
		Reason:             string(userv1.Ready),
		Message:            "sync namespace successfully",
	}
	condition := helper.GetCondition(userNamespace.Status.Conditions, nsCondition)
	defer func() {
		if helper.DiffCondition(condition, nsCondition) {
			r.saveCondition(userNamespace, nsCondition.DeepCopy())
		}
	}()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var change controllerutil.OperationResult
		var err error
		ns := &v1.Namespace{}
		ns.Name = config.GetUserNamespace(userNamespace.Name)
		ns.Labels = map[string]string{}
		if err = r.Get(ctx, client.ObjectKeyFromObject(ns), ns); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		if !ns.CreationTimestamp.IsZero() {
			r.Logger.V(1).Info("define namespace User namespace is created", "isCreated", true, "namespace", ns.Name)
		}
		if change, err = controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
			if ns.Annotations == nil {
				ns.Annotations = make(map[string]string)
			}
			ns.Annotations[userAnnotationCreatorKey] = userNamespace.Spec.Creator
			if userNamespace.Annotations[userAnnotationOwnerKey] == "" {
				userNamespace.Annotations[userAnnotationOwnerKey] = userNamespace.Spec.Creator
			}
			ns.Annotations[userAnnotationOwnerKey] = userNamespace.Annotations[userAnnotationOwnerKey]
			ns.Labels = config.SetPodSecurity(ns.Labels)
			ns.Labels[userLabelOwnerKey] = userNamespace.Annotations[userAnnotationOwnerKey]
			ns.SetOwnerReferences([]metav1.OwnerReference{})
			return controllerutil.SetControllerReference(userNamespace, ns, r.Scheme)
		}); err != nil {
			return fmt.Errorf("unable to update namespace by User: %w", err)
		}
		r.Logger.V(1).Info("create or update namespace by User", "OperationResult", change)
		nsCondition.Message = fmt.Sprintf("sync namespace %s/%s successfully", ns.Name, ns.ResourceVersion)
		return nil
	}); err != nil {
		helper.SetConditionError(nsCondition, "SyncUserError", err)
		r.Recorder.Eventf(userNamespace, v1.EventTypeWarning, "syncUser", "Sync User namespace %s is error: %v", userNamespace.Name, err)
	}
	return ctx
}

func (r *UserNamespaceReconciler) syncRole(ctx context.Context, userNamespace *userv1.UserNamespace) context.Context {
	roleConditionType := userv1.ConditionType("RoleSyncReady")
	roleCondition := &userv1.Condition{
		Type:               roleConditionType,
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
		Reason:             string(userv1.Ready),
		Message:            "sync namespace role successfully",
	}
	condition := helper.GetCondition(userNamespace.Status.Conditions, roleCondition)
	defer func() {
		if helper.DiffCondition(condition, roleCondition) {
			r.saveCondition(userNamespace, roleCondition.DeepCopy())
		}
	}()
	//create three roles
	r.createRole(ctx, roleCondition, userNamespace, userv1.OwnerRoleType)
	r.createRole(ctx, roleCondition, userNamespace, userv1.ManagerRoleType)
	r.createRole(ctx, roleCondition, userNamespace, userv1.DeveloperRoleType)

	return ctx
}

func (r *UserNamespaceReconciler) createRole(ctx context.Context, condition *userv1.Condition, userNamespace *userv1.UserNamespace, roleType userv1.RoleType) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var change controllerutil.OperationResult
		var err error
		role := &rbacv1.Role{}
		role.Name = string(roleType)
		role.Namespace = config.GetUserNamespace(userNamespace.Name)
		role.Labels = map[string]string{}
		if change, err = controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
			role.Annotations = map[string]string{
				userAnnotationCreatorKey: userNamespace.Spec.Creator,
			}
			role.Rules = config.GetUserRole(roleType)
			return controllerutil.SetControllerReference(userNamespace, role, r.Scheme)
		}); err != nil {
			return fmt.Errorf("unable to create namespace role by User: %w", err)
		}
		r.Logger.V(1).Info("create or update namespace role  by User", "OperationResult", change)
		condition.Message = fmt.Sprintf("sync namespace role %s/%s successfully", role.Name, role.ResourceVersion)
		return nil
	}); err != nil {
		helper.SetConditionError(condition, "SyncUserError", err)
		r.Recorder.Eventf(userNamespace, v1.EventTypeWarning, "syncUserRole", "Sync User namespace role %s is error: %v", userNamespace.Name, err)
	}
}

func (r *UserNamespaceReconciler) syncRoleBinding(ctx context.Context, userNamespace *userv1.UserNamespace) context.Context {
	owner := userNamespace.Annotations[userAnnotationOwnerKey]
	if owner == "" {
		return ctx
	}
	roleBindingConditionType := userv1.ConditionType("RoleBindingSyncReady")
	rbCondition := &userv1.Condition{
		Type:               roleBindingConditionType,
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
		Reason:             string(userv1.Ready),
		Message:            "sync namespace role binding successfully",
	}
	condition := helper.GetCondition(userNamespace.Status.Conditions, rbCondition)
	defer func() {
		if helper.DiffCondition(condition, rbCondition) {
			r.saveCondition(userNamespace, rbCondition.DeepCopy())
		}
	}()

	if err := r.createRoleBinding(ctx, condition, userNamespace, userv1.OwnerRoleType, owner); err != nil {
		helper.SetConditionError(condition, "SyncUserError", err)
		r.Recorder.Eventf(userNamespace, v1.EventTypeWarning, "syncUserRoleBinding", "Sync User namespace role binding %s is error: %v", userNamespace.Name, err)
	}

	return ctx
}

func (r *UserNamespaceReconciler) createRoleBinding(ctx context.Context, condition *userv1.Condition, userNamespace *userv1.UserNamespace, roleType userv1.RoleType, userName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var change controllerutil.OperationResult
		var err error
		roleBinding := &rbacv1.RoleBinding{}
		roleBinding.Name = config.GetGroupRoleBindingName(userName)
		roleBinding.Namespace = config.GetUserNamespace(userNamespace.Name)
		roleBinding.Labels = map[string]string{}
		if change, err = controllerutil.CreateOrUpdate(ctx, r.Client, roleBinding, func() error {
			roleBinding.Annotations = map[string]string{
				userAnnotationCreatorKey: userNamespace.Spec.Creator,
				userAnnotationOwnerKey:   userName,
			}
			roleBinding.RoleRef = rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     string(roleType),
			}
			roleBinding.Subjects = config.GetUsersSubject(userName)
			return controllerutil.SetControllerReference(userNamespace, roleBinding, r.Scheme)
		}); err != nil {
			return fmt.Errorf("unable to create namespace role binding by User: %w", err)
		}
		r.Logger.V(1).Info("create or update namespace role binding by User", "OperationResult", change)
		condition.Message = fmt.Sprintf("sync namespace role binding %s/%s successfully", roleBinding.Name, roleBinding.ResourceVersion)
		return nil
	})
}

func (r *UserNamespaceReconciler) saveCondition(userNamespace *userv1.UserNamespace, condition *userv1.Condition) {
	userNamespace.Status.Conditions = helper.UpdateCondition(userNamespace.Status.Conditions, *condition)
}

func (r *UserNamespaceReconciler) syncFinalStatus(ctx context.Context, userNamespace *userv1.UserNamespace) context.Context {
	condition := &userv1.Condition{
		Type:               userv1.Ready,
		Status:             v1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
		Reason:             string(userv1.Ready),
		Message:            "UserNamespace is available now",
	}
	defer r.saveCondition(userNamespace, condition)

	if !helper.IsConditionsTrue(userNamespace.Status.Conditions) {
		condition.LastHeartbeatTime = metav1.Now()
		condition.Status = v1.ConditionFalse
		condition.Reason = "Not" + string(userv1.Ready)
		condition.Message = "UserNamespace is not available now"
		userNamespace.Status.Phase = userv1.UserNamespaceUnknown
	} else {
		userNamespace.Status.Phase = userv1.UserNamespaceReady
	}
	return ctx
}

func (r *UserNamespaceReconciler) updateStatus(ctx context.Context, nn types.NamespacedName, status *userv1.UserNamespaceStatus) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		original := &userv1.UserNamespace{}
		if err := r.Get(ctx, nn, original); err != nil {
			return err
		}
		original.Status = *status
		return r.Client.Status().Update(ctx, original)
	})
}
