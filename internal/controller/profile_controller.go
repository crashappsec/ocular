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

// ProfileReconciler reconciles a Profile object
type ProfileReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=profiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=profiles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=profiles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The profile controller is responsible for ensuring that the profile is valid
// by checking that all referenced uploaders exist and that required parameters are provided.
// It also ensures that there are no duplicate uploader references
// in the profile spec. If the profile is valid, it updates the status accordingly.
// If the profile is invalid, it sets the status to invalid with appropriate conditions.
func (r *ProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// Fetch the profile instance
	profile := &v1.Profile{}
	err := r.Get(ctx, req.NamespacedName, profile)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	l.Info("handling finalizers for profile", "name", profile.GetName())

	finalized, err := resources.PerformFinalizer(ctx, profile, "profile.finalizers.ocular.crashoverride.run/cleanup", nil)

	if err != nil {
		l.Error(err, "error performing finalizer for profile", "name", profile.GetName())
		return ctrl.Result{}, err
	} else if finalized {
		if err = r.Update(ctx, profile); err != nil {
			l.Error(err, "error updating profile after finalizer handling", "name", profile.GetName())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	duplicates := make(map[string]struct{}, len(profile.Spec.UploaderRefs))
	for _, uploaderRef := range profile.Spec.UploaderRefs {
		if _, exists := duplicates[uploaderRef.Name]; exists {
			// Duplicate uploader reference found
			profile.Status.Valid = false
			profile.Status.Conditions = []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "InvalidSpec",
					Message:            "Duplicate uploader reference found: " + uploaderRef.Name,
					LastTransitionTime: metav1.NewTime(time.Now()),
				},
			}
			if err := r.Status().Update(ctx, profile); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		duplicates[uploaderRef.Name] = struct{}{}
		uploader := &v1.Uploader{}
		err := r.Get(ctx, client.ObjectKey{
			Name:      uploaderRef.Name,
			Namespace: profile.Namespace,
		}, uploader)
		if err != nil {
			if errors.IsNotFound(err) {
				// Uploader not found
				profile.Status.Valid = false
				profile.Status.Conditions = []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						Reason:             "InvalidSpec",
						Message:            "Referenced uploader not found: " + uploaderRef.Name,
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				if err := r.Status().Update(ctx, profile); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		for paramName, paramDef := range uploader.Spec.Parameters {
			if _, exists := uploaderRef.Parameters[paramName]; !exists && paramDef.Required {
				// Required parameter missing in profile
				profile.Status.Valid = false
				profile.Status.Conditions = []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						Reason:             "InvalidSpec",
						Message:            "Required parameter missing for uploader " + uploaderRef.Name + ": " + paramName,
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				if err := r.Status().Update(ctx, profile); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}
	}

	if !profile.Status.Valid {
		profile.Status.Valid = true

		profile.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Reconciled",
				Message:            "Profile is ready and its spec is valid.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		if err := r.Status().Update(ctx, profile); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Profile{}).
		Named("profile").
		Complete(r)
}
