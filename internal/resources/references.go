// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/crashappsec/ocular/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Invocation represents the invocation of an Ocular resource
// (profile, downloader, crawler, uploader). It contains the spec
// of the resource, the metadata and the parameters that should be set.
// This is done since some resources additionally have a cluster wide
// version that share the same spec.
type Invocation[S any] struct {
	Spec       S
	Parameters []v1beta1.ParameterSetting
	Metadata   metav1.ObjectMeta
}

type InvalidObjectReference struct {
	Message string
}

func (i InvalidObjectReference) Error() string {
	return i.Message
}

func UploaderInvocationFromReference(ctx context.Context, c client.Client, namespace string, ref v1beta1.ParameterizedObjectReference) (Invocation[v1beta1.UploaderSpec], error) {
	var (
		err error
		r   Invocation[v1beta1.UploaderSpec]
	)
	switch ref.Kind {
	case "Uploader", "":
		if ref.Namespace != "" && namespace != ref.Namespace {
			err = InvalidObjectReference{
				Message: fmt.Sprintf("invalid namespace '%s', reference should same as parent namespace '%s' or empty", ref.Namespace, namespace),
			}
		} else {
			var u v1beta1.Uploader
			err = c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &u)
			r = Invocation[v1beta1.UploaderSpec]{
				Spec:       u.Spec,
				Metadata:   u.ObjectMeta,
				Parameters: ref.Parameters,
			}
		}
	case "ClusterUploader":
		var u v1beta1.ClusterUploader
		err = c.Get(ctx, client.ObjectKey{Name: ref.Name}, &u)
		r = Invocation[v1beta1.UploaderSpec]{
			Spec:       u.Spec,
			Metadata:   u.ObjectMeta,
			Parameters: ref.Parameters,
		}
	default:
		err = InvalidObjectReference{
			Message: fmt.Sprintf("invalid kind for uploader ref '%s', should either be 'Uploader' or 'ClusterUploader'", ref.Kind),
		}
	}

	return r, err
}

func ProfileInvocationFromReference(ctx context.Context, c client.Client, namespace string, ref v1beta1.ParameterizedObjectReference) (Invocation[v1beta1.ProfileSpec], error) {
	var (
		p v1beta1.Profile
		r Invocation[v1beta1.ProfileSpec]
	)
	if ref.Kind != "Profile" && ref.Kind != "" {
		return r, InvalidObjectReference{
			Message: fmt.Sprintf("invalid kind for crawler reference '%s', should either be 'Crawler' or 'ClusterCrawler'", ref.Kind),
		}
	}

	if ref.Namespace != "" && namespace != ref.Namespace {
		return r, InvalidObjectReference{
			Message: fmt.Sprintf("invalid namespace '%s', reference should same as parent namespace '%s' or empty", ref.Namespace, namespace),
		}
	}

	err := c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &p)
	r = Invocation[v1beta1.ProfileSpec]{
		Spec:       p.Spec,
		Metadata:   p.ObjectMeta,
		Parameters: ref.Parameters,
	}
	return r, err
}

func DownloaderInvocationFromReference(ctx context.Context, c client.Client, namespace string, ref v1beta1.ParameterizedObjectReference) (Invocation[v1beta1.DownloaderSpec], error) {
	var (
		err error
		r   Invocation[v1beta1.DownloaderSpec]
	)
	switch ref.Kind {
	case "Downloader", "":
		if ref.Namespace != "" && namespace != ref.Namespace {
			err = InvalidObjectReference{
				Message: fmt.Sprintf("invalid namespace '%s', reference should same as parent namespace '%s' or empty", ref.Namespace, namespace),
			}
		} else {
			var u v1beta1.Downloader
			err = c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &u)
			r = Invocation[v1beta1.DownloaderSpec]{
				Spec:       u.Spec,
				Metadata:   u.ObjectMeta,
				Parameters: ref.Parameters,
			}
		}
	case "ClusterDownloader":
		var u v1beta1.ClusterDownloader
		err = c.Get(ctx, client.ObjectKey{Name: ref.Name}, &u)
		r = Invocation[v1beta1.DownloaderSpec]{
			Spec:       u.Spec,
			Metadata:   u.ObjectMeta,
			Parameters: ref.Parameters,
		}
	default:
		err = InvalidObjectReference{
			Message: fmt.Sprintf("invalid kind for downloader reference '%s', should either be 'Downloader' or 'ClusterDownloader'", ref.Kind),
		}
	}

	return r, err
}

func CrawlerInvocationFromReference(ctx context.Context, c client.Client, namespace string, ref v1beta1.ParameterizedObjectReference) (Invocation[v1beta1.CrawlerSpec], error) {
	var (
		err error
		r   Invocation[v1beta1.CrawlerSpec]
	)
	switch ref.Kind {
	case "Crawler", "":
		if ref.Namespace != "" && namespace != ref.Namespace {
			err = InvalidObjectReference{
				Message: fmt.Sprintf("invalid namespace '%s', reference should same as parent namespace '%s' or empty", ref.Namespace, namespace),
			}
		} else {
			var u v1beta1.Crawler
			err = c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &u)
			r = Invocation[v1beta1.CrawlerSpec]{
				Spec:       u.Spec,
				Metadata:   u.ObjectMeta,
				Parameters: ref.Parameters,
			}

		}
	case "ClusterCrawler":
		var u v1beta1.ClusterCrawler
		err = c.Get(ctx, client.ObjectKey{Name: ref.Name}, &u)
		r = Invocation[v1beta1.CrawlerSpec]{
			Spec:       u.Spec,
			Metadata:   u.ObjectMeta,
			Parameters: ref.Parameters,
		}
	default:
		err = InvalidObjectReference{
			Message: fmt.Sprintf("invalid kind for crawler reference '%s', should either be 'Crawler' or 'ClusterCrawler'", ref.Kind),
		}
	}

	return r, err
}

func ReferenceDefaulter(ref v1beta1.ParameterizedObjectReference, defaultKind, defaultNamespace string) v1beta1.ParameterizedObjectReference {
	if ref.Kind == "" {
		ref.Kind = defaultKind
	}

	if ref.Namespace == "" && !strings.HasPrefix(ref.Kind, "Cluster") {
		ref.Namespace = defaultNamespace
	}
	return ref
}
