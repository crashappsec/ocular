// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"context"
	"time"

	"github.com/crashappsec/ocular/internal/resources"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/crashappsec/ocular/api/v1"
)

// DownloaderReconciler reconciles a Downloader object
type DownloaderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=downloaders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=downloaders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=downloaders/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Downloader object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *DownloaderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// Fetch the Downloader instance
	downloader := &v1.Downloader{}
	err := r.Get(ctx, req.NamespacedName, downloader)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	finalized, err := resources.PerformFinalizer(ctx, downloader, "downloader.finalizers.ocular.crashoverride.run/cleanup", nil)
	if err != nil {
		l.Error(err, "error performing finalizer for downloader", "name", downloader.GetName())
		return ctrl.Result{}, err
	} else if finalized {
		if err = r.Update(ctx, downloader); err != nil {
			l.Error(err, "error updating downloader after finalizer handling", "name", downloader.GetName())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !downloader.Status.Valid {
		downloader.Status.Valid = true

		downloader.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Reconciled",
				Message:            "Downloader is ready and its spec is valid.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		if err := r.Status().Update(ctx, downloader); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DownloaderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Downloader{}).
		Named("downloader").
		Complete(r)
}
