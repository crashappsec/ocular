package resources

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileFunc[T client.Object] func(ctx context.Context, c client.Client, actual, desired T) error

func ReconcileChildResource[T client.Object](ctx context.Context, c client.Client, desired client.Object, owner client.Object, scheme *runtime.Scheme, reconciler ReconcileFunc[T]) error {
	if desired == nil {
		return nil
	}

	if err := ctrl.SetControllerReference(owner, desired, scheme); err != nil {
		return err
	}

	found := desired.DeepCopyObject().(client.Object)

	// Check if the child resource already exists.
	err := c.Get(ctx, types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		return c.Create(ctx, desired)
	} else if err != nil {
		return err
	}

	if reconciler != nil {
		return reconciler(ctx, c, found.(T), desired.(T))
	}

	return nil
}
