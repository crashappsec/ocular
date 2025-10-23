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
	"fmt"
	"sort"
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ref "k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

// realClock is the real implementation of Clock
// This is done to allow for easier testing
type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() } //nolint:staticcheck

// Clock knows how to get the current time.
// It can be used to fake out timing for testing.
type Clock interface {
	Now() time.Time
}

// CronSearchReconciler reconciles a CronSearch object
type CronSearchReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	SearchClusterRole string
	Clock
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=cronsearches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=cronsearches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=cronsearches/finalizers,verbs=update
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches/status,verbs=get

var (
	scheduledTimeAnnotation = "ocular.crashoverride.run/scheduled-at"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// This function is designed to behave the same as [k8s.io/api/batch/v1.CronJob] controller,
// The actual implementation is copied and modified from the kubebuilder tutorial, where
// a CronJob controller is implemented.
// https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/project/internal/controller/cronjob_controller.go
// why re-invent the wheel?
func (r *CronSearchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var cronSearch v1beta1.CronSearch
	if err := r.Get(ctx, req.NamespacedName, &cronSearch); err != nil {
		log.Error(err, "unable to fetch CronSearch")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childSearches v1beta1.SearchList
	if err := r.List(ctx, &childSearches, client.InNamespace(req.Namespace), client.MatchingFields{searchOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Searches")
		return ctrl.Result{}, err
	}

	categorizedChildSearches, mostRecentTime := findExistingSearches(ctx, &childSearches)

	if mostRecentTime != nil {
		cronSearch.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		cronSearch.Status.LastScheduleTime = nil
	}
	cronSearch.Status.Active = nil
	for _, activeSearch := range categorizedChildSearches.active {
		searchRef, err := ref.GetReference(r.Scheme, activeSearch)
		if err != nil {
			log.Error(err, "unable to make reference to active search", "search", activeSearch)
			continue
		}
		cronSearch.Status.Active = append(cronSearch.Status.Active, *searchRef)
	}

	log.V(1).Info("search count",
		"active searches", len(categorizedChildSearches.active),
		"successful searches", len(categorizedChildSearches.successful),
		"failed searches", len(categorizedChildSearches.failed))

	if err := r.Status().Update(ctx, &cronSearch); err != nil {
		log.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, err
	}

	if cronSearch.Spec.FailedJobsHistoryLimit != nil {
		sort.Slice(categorizedChildSearches.failed, func(i, j int) bool {
			if categorizedChildSearches.failed[i].Status.StartTime == nil {
				return categorizedChildSearches.failed[j].Status.StartTime != nil
			}
			return categorizedChildSearches.failed[i].Status.StartTime.Before(categorizedChildSearches.failed[j].Status.StartTime)
		})
		for i, search := range categorizedChildSearches.failed {
			if int32(i) >= int32(len(categorizedChildSearches.failed))-*cronSearch.Spec.FailedJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, search, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete old failed search", "search", search)
			} else {
				log.V(0).Info("deleted old failed search", "search", search)
			}
		}
	}

	if cronSearch.Spec.SuccessfulJobsHistoryLimit != nil {
		sort.Slice(categorizedChildSearches.successful, func(i, j int) bool {
			if categorizedChildSearches.successful[i].Status.StartTime == nil {
				return categorizedChildSearches.successful[j].Status.StartTime != nil
			}
			return categorizedChildSearches.successful[i].Status.StartTime.Before(categorizedChildSearches.successful[j].Status.StartTime)
		})
		for i, search := range categorizedChildSearches.successful {
			if int32(i) >= int32(len(categorizedChildSearches.successful))-*cronSearch.Spec.SuccessfulJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, search, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				log.Error(err, "unable to delete old successful search", "search", search)
			} else {
				log.V(0).Info("deleted old successful search", "search", search)
			}
		}
	}

	if cronSearch.Spec.Suspend != nil && *cronSearch.Spec.Suspend {
		log.V(1).Info("cronsearch suspended, skipping")
		return ctrl.Result{}, nil
	}

	missedRun, nextRun, err := getNextSchedule(&cronSearch, r.Now())
	if err != nil {
		log.Error(err, "unable to figure out CronSearch schedule")
		return ctrl.Result{}, nil
	}

	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())} // save this so we can re-use it elsewhere
	log = log.WithValues("now", r.Now(), "next run", nextRun)

	if missedRun.IsZero() {
		log.V(1).Info("no upcoming scheduled times, sleeping until next")
		return scheduledResult, nil
	}

	log = log.WithValues("current run", missedRun)
	tooLate := false
	if cronSearch.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*cronSearch.Spec.StartingDeadlineSeconds) * time.Second).Before(r.Now())
	}
	if tooLate {
		log.V(1).Info("missed starting deadline for last run, sleeping till next")
		return scheduledResult, nil
	}

	if cronSearch.Spec.ConcurrencyPolicy == v1beta1.ForbidConcurrent && len(categorizedChildSearches.active) > 0 {
		log.V(1).Info("concurrency policy blocks concurrent runs, skipping", "num active", len(categorizedChildSearches.active))
		return scheduledResult, nil
	}

	if cronSearch.Spec.ConcurrencyPolicy == v1beta1.ReplaceConcurrent {
		for _, activeSearch := range categorizedChildSearches.active {
			// we don't care if the search was already deleted
			if err := r.Delete(ctx, activeSearch, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete active search", "search", activeSearch)
				return ctrl.Result{}, err
			}
		}
	}

	search, err := constructSearchForCronSearch(&cronSearch, missedRun, r.Scheme)
	if err != nil {
		log.Error(err, "unable to construct search from template")
		return scheduledResult, nil
	}

	if err := r.Create(ctx, search); err != nil {
		log.Error(err, "unable to Search Job for CronSearch", "search", search)
		return ctrl.Result{}, err
	}

	log.V(1).Info("created Search for CronSearch run", "search", search)

	return scheduledResult, nil
}

func isSearchFinished(search *v1beta1.Search) (bool, metav1.ConditionStatus) {
	for _, c := range search.Status.Conditions {
		if c.Type == v1beta1.CompletedSuccessfullyConditionType {
			return true, c.Status
		}
	}

	return false, ""
}

func getScheduledTimeForSearch(search *v1beta1.Search) (*time.Time, error) {
	timeRaw := search.Annotations[scheduledTimeAnnotation]
	if len(timeRaw) == 0 {
		return nil, nil
	}

	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return nil, err
	}
	return &timeParsed, nil
}

func getNextSchedule(cronSearch *v1beta1.CronSearch, now time.Time) (lastMissed time.Time, next time.Time, err error) {
	sched, err := cron.ParseStandard(cronSearch.Spec.Schedule)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("unparseable schedule %q: %w", cronSearch.Spec.Schedule, err)
	}

	// for optimization purposes, cheat a bit and start from our last observed run time
	// we could reconstitute this here, but there's not much point, since we've
	// just updated it.
	var earliestTime time.Time
	if cronSearch.Status.LastScheduleTime != nil {
		earliestTime = cronSearch.Status.LastScheduleTime.Time
	} else {
		earliestTime = cronSearch.CreationTimestamp.Time
	}
	if cronSearch.Spec.StartingDeadlineSeconds != nil {
		// controller is not going to schedule anything below this point
		schedulingDeadline := now.Add(-time.Second * time.Duration(*cronSearch.Spec.StartingDeadlineSeconds))

		if schedulingDeadline.After(earliestTime) {
			earliestTime = schedulingDeadline
		}
	}
	if earliestTime.After(now) {
		return time.Time{}, sched.Next(now), nil
	}

	starts := 0
	for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
		lastMissed = t
		// An object might miss several starts. For example, if
		// controller gets wedged on Friday at 5:01pm when everyone has
		// gone home, and someone comes in on Tuesday AM and discovers
		// the problem and restarts the controller, then all the hourly
		// searches, more than 80 of them for one hourly scheduledJob, should
		// all start running with no further intervention (if the scheduledJob
		// allows concurrency and late starts).
		//
		// However, if there is a bug somewhere, or incorrect clock
		// on controller's server or apiservers (for setting creationTimestamp)
		// then there could be so many missed start times (it could be off
		// by decades or more), that it would eat up all the CPU and memory
		// of this controller. In that case, we want to not try to list
		// all the missed start times.
		starts++
		if starts > 100 {
			// We can't get the most recent times so just return an empty slice
			return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100). Set or decrease .spec.startingDeadlineSeconds or check clock skew.") //nolint:staticcheck
		}
	}
	return lastMissed, sched.Next(now), nil
}

func constructSearchForCronSearch(cronSearch *v1beta1.CronSearch, scheduledTime time.Time, scheme *runtime.Scheme) (*v1beta1.Search, error) {
	// We want search names for a given nominal start time to have a deterministic name to avoid the same search being created twice
	name := fmt.Sprintf("%s-%d", cronSearch.Name, scheduledTime.Unix())

	searchSpec := *cronSearch.Spec.SearchTemplate.Spec.DeepCopy()
	// this is a bug, using TTL seconds after finished here would cause the search to be deleted too early
	// we need to use our own cleanup logic based on the history limits
	searchSpec.TTLSecondsAfterFinished = nil
	search := &v1beta1.Search{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   cronSearch.Namespace,
		},
		Spec: searchSpec,
	}
	for k, v := range cronSearch.Spec.SearchTemplate.Annotations {
		search.Annotations[k] = v
	}
	search.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
	for k, v := range cronSearch.Spec.SearchTemplate.Labels {
		search.Labels[k] = v
	}
	if err := ctrl.SetControllerReference(cronSearch, search, scheme); err != nil {
		return nil, err
	}

	return search, nil
}

type categorizedSearches struct {
	active     []*v1beta1.Search
	failed     []*v1beta1.Search
	successful []*v1beta1.Search
}

func findExistingSearches(ctx context.Context, childSearches *v1beta1.SearchList) (categorizedSearches, *time.Time) {
	log := logf.FromContext(ctx)
	// find the active list of search
	var activeSearches []*v1beta1.Search
	var successfulSearches []*v1beta1.Search
	var failedSearches []*v1beta1.Search
	var mostRecentTime *time.Time // find the last run so we can update the status

	for i, search := range childSearches.Items {
		finished, finishedType := isSearchFinished(&search)
		switch {
		case !finished: // ongoing
			activeSearches = append(activeSearches, &childSearches.Items[i])
		case finishedType == metav1.ConditionFalse: // failed
			failedSearches = append(failedSearches, &childSearches.Items[i])
		case finishedType == metav1.ConditionTrue: // succeeded
			successfulSearches = append(successfulSearches, &childSearches.Items[i])
		}

		scheduledTimeForSearch, err := getScheduledTimeForSearch(&search)
		if err != nil {
			log.Error(err, "unable to parse schedule time for child search", "search", &search)
			continue
		}
		if scheduledTimeForSearch != nil {
			if mostRecentTime == nil || mostRecentTime.Before(*scheduledTimeForSearch) {
				mostRecentTime = scheduledTimeForSearch
			}
		}
	}

	return categorizedSearches{
		active:     activeSearches,
		failed:     failedSearches,
		successful: successfulSearches,
	}, mostRecentTime
}

var (
	searchOwnerKey = "status.cronSearchControllerName"
	apiGVStr       = v1beta1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *CronSearchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Clock == nil {
		r.Clock = realClock{}
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1beta1.Search{}, searchOwnerKey, func(rawObj client.Object) []string {
		search := rawObj.(*v1beta1.Search)
		owner := metav1.GetControllerOf(search)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "CronSearch" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.CronSearch{}).
		Owns(&v1beta1.Search{}).
		Named("cronsearch").
		Complete(r)
}
