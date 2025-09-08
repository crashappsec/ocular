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

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/resources"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// CrawlerReconciler reconciles a Crawler object
type CrawlerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=crawlers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=crawlers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=crawlers/finalizers,verbs=update

func (r *CrawlerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// Fetch the crawler instance
	crawler := &v1beta1.Crawler{}
	err := r.Get(ctx, req.NamespacedName, crawler)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	finalized, err := resources.PerformFinalizer(ctx, crawler, "crawler.finalizers.ocular.crashoverride.run/cleanup", nil)
	if err != nil {
		l.Error(err, "error performing finalizer for crawler", "name", crawler.GetName())
		return ctrl.Result{}, err
	} else if finalized {
		if err = r.Update(ctx, crawler); err != nil {
			l.Error(err, "error updating crawler after finalizer handling", "name", crawler.GetName())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !crawler.Status.Valid {
		crawler.Status.Valid = true
		crawler.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Reconciled",
				Message:            "Crawler is ready and its spec is valid.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		if err := r.Status().Update(ctx, crawler); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CrawlerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Crawler{}).
		Named("crawler").
		Complete(r)
}
