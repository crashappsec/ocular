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

	"github.com/crashappsec/ocular/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InvalidObjectReference struct {
	Message string
}

func (i InvalidObjectReference) Error() string {
	return i.Message
}

func UploaderSpecFromReference(ctx context.Context, c client.Client, namespace string, ref corev1.ObjectReference) (v1beta1.UploaderSpec, error) {
	var (
		spec v1beta1.UploaderSpec
		err  error
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
			spec = u.Spec
		}
	case "ClusterUploader":
		var u v1beta1.ClusterUploader
		err = c.Get(ctx, client.ObjectKey{Name: ref.Name}, &u)
		spec = u.Spec
	default:
		err = InvalidObjectReference{
			Message: fmt.Sprintf("invalid kind for uploader ref '%s', should either be 'Uploader' or 'ClusterUploader'", ref.Kind),
		}
	}

	return spec, err
}

func DownloaderSpecFromReference(ctx context.Context, c client.Client, namespace string, ref corev1.ObjectReference) (v1beta1.DownloaderSpec, error) {
	var (
		spec v1beta1.DownloaderSpec
		err  error
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
			spec = u.Spec
		}
	case "ClusterDownloader":
		var u v1beta1.ClusterDownloader
		err = c.Get(ctx, client.ObjectKey{Name: ref.Name}, &u)
		spec = u.Spec
	default:
		err = InvalidObjectReference{
			Message: fmt.Sprintf("invalid kind for downloader reference '%s', should either be 'Downloader' or 'ClusterDownloader'", ref.Kind),
		}
	}

	return spec, err
}

func CrawlerSpecFromReference(ctx context.Context, c client.Client, namespace string, ref corev1.ObjectReference) (v1beta1.CrawlerSpec, error) {
	var (
		spec v1beta1.CrawlerSpec
		err  error
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
			spec = u.Spec
		}
	case "ClusterCrawler":
		var u v1beta1.ClusterCrawler
		err = c.Get(ctx, client.ObjectKey{Name: ref.Name}, &u)
		spec = u.Spec
	default:
		err = InvalidObjectReference{
			Message: fmt.Sprintf("invalid kind for crawler reference '%s', should either be 'Crawler' or 'ClusterCrawler'", ref.Kind),
		}
	}

	return spec, err
}
