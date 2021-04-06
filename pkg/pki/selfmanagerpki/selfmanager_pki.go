package selfmanagerpki

import (
	"context"
	"fmt"
	"github.com/Orange-OpenSource/nifikop/api/v1alpha1"
	pkicommon "github.com/Orange-OpenSource/nifikop/pkg/util/pki"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (s *SelfManager) ReconcilePKI(ctx context.Context, logger logr.Logger, scheme *runtime.Scheme, externalHostnames []string) error {
	logger.Info("Reconciling selfmanager PKI")

	resources := s.fullPKI(s.cluster, scheme, externalHostnames)

	for _, o := range resources {
		if err := reconcile(ctx, logger, s.client, o, s.cluster); err != nil {
			return err
		}
	}

	return nil
	// TODO Generate all certs
	// TODO Setup all secrets from certs
}

func (s *SelfManager) fullPKI(cluster *v1alpha1.NifiCluster, scheme *runtime.Scheme, externalHostnames []string) []runtime.Object {
	var objects []runtime.Object

	// TODO no need ?
	//objects = append(objects, []runtime.Object{
	//	// A self-signer for the CA Certificate
	//	selfSignerForNamespace(cluster, scheme),
	//	// The CA Certificate
	//	caCertForNamespace(cluster, scheme),
	//	// A issuer backed by the CA certificate - so it can provision secrets
	//	// in this namespace
	//	mainIssuerForNamespace(cluster, scheme),
	//}...,
	//)

	objects = append(objects, pkicommon.ControllerUserForCluster(cluster))
	// Node "users"
	for _, user := range pkicommon.NodeUsersForCluster(cluster, externalHostnames) {
		objects = append(objects, user)
	}
	return objects
}

func (s SelfManager) FinalizePKI(ctx context.Context, logger logr.Logger) error {
	logger.Info("Removing selfmanager certificates and secrets")

	// Safety check that we are actually doing something
	if s.cluster.Spec.ListenersConfig.SSLSecrets == nil {
		return nil
	}

	if s.cluster.Spec.ListenersConfig.SSLSecrets.Create {
		// Names of our certificates and secrets
		objNames := []types.NamespacedName{
			{Name: fmt.Sprintf(pkicommon.NodeControllerTemplate, s.cluster.Name), Namespace: s.cluster.Namespace},
		}

		for _, node := range s.cluster.Spec.Nodes {
			objNames = append(objNames, types.NamespacedName{Name: fmt.Sprintf(pkicommon.NodeServerCertTemplate, s.cluster.Name, node.Id), Namespace: s.cluster.Namespace})
		}

		if s.cluster.Spec.ListenersConfig.SSLSecrets.IssuerRef == nil {
			objNames = append(
				objNames,
				types.NamespacedName{Name: fmt.Sprintf(pkicommon.NodeCACertTemplate, s.cluster.Name), Namespace: s.cluster.Namespace})

		}
		for _, obj := range objNames {
			//// Delete the certificates first so we don't accidentally recreate the
			//// secret after it gets deleted
			//cert := &certv1.Certificate{}
			//if err := s.client.Get(ctx, obj, cert); err != nil {
			//	if apierrors.IsNotFound(err) {
			//		continue
			//	} else {
			//		return err
			//	}
			//}
			//if err := s.client.Delete(ctx, cert); err != nil {
			//	return err
			//}

			// Might as well delete the secret and leave the controller reference earlier
			// as a safety belt
			secret := &corev1.Secret{}
			if err := s.client.Get(ctx, obj, secret); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				} else {
					return err
				}
			}
			if err := s.client.Delete(ctx, secret); err != nil {
				return err
			}
		}
	}

	return nil
}
