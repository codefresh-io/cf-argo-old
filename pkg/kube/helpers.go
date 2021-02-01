package kube

// import (
// 	"k8s.io/apimachinery/pkg/runtime"
// 	appsv1 "k8s.io/api/apps/v1"
// 	appsv1beta1 "k8s.io/api/apps/v1beta1"
// 	appsv1beta2 "k8s.io/api/apps/v1beta2"
// 	batchv1 "k8s.io/api/batch/v1"
// 	corev1 "k8s.io/api/core/v1"
// 	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
// 	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
// 	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/client-go/kubernetes/scheme"
// )

// func (c *Client) isReady(o info) (bool, error) {
// 	switch o.(type) {
// 	case *corev1.Pod:
// 		pod := &corev1.Pod{}
// 		if err := scheme.Scheme.Convert(o, pod, nil) {}
// 		pod, err := c.CoreV1().Pods(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
// 		if err != nil || !w.isPodReady(pod) {
// 			return false, err
// 		}
// 	case *batchv1.Job:
// 		if waitForJobsEnabled {
// 			job, err := w.c.BatchV1().Jobs(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
// 			if err != nil || !w.jobReady(job) {
// 				return false, err
// 			}
// 		}
// 	case *appsv1.Deployment, *appsv1beta1.Deployment, *appsv1beta2.Deployment, *extensionsv1beta1.Deployment:
// 		currentDeployment, err := w.c.AppsV1().Deployments(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
// 		if err != nil {
// 			return false, err
// 		}
// 		// If paused deployment will never be ready
// 		if currentDeployment.Spec.Paused {
// 			continue
// 		}
// 		// Find RS associated with deployment
// 		newReplicaSet, err := deploymentutil.GetNewReplicaSet(currentDeployment, w.c.AppsV1())
// 		if err != nil || newReplicaSet == nil {
// 			return false, err
// 		}
// 		if !w.deploymentReady(newReplicaSet, currentDeployment) {
// 			return false, nil
// 		}
// 	case *corev1.PersistentVolumeClaim:
// 		claim, err := w.c.CoreV1().PersistentVolumeClaims(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
// 		if err != nil {
// 			return false, err
// 		}
// 		if !w.volumeReady(claim) {
// 			return false, nil
// 		}
// 	case *corev1.Service:
// 		svc, err := w.c.CoreV1().Services(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
// 		if err != nil {
// 			return false, err
// 		}
// 		if !w.serviceReady(svc) {
// 			return false, nil
// 		}
// 	case *extensionsv1beta1.DaemonSet, *appsv1.DaemonSet, *appsv1beta2.DaemonSet:
// 		ds, err := w.c.AppsV1().DaemonSets(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
// 		if err != nil {
// 			return false, err
// 		}
// 		if !w.daemonSetReady(ds) {
// 			return false, nil
// 		}
// 	case *apiextv1beta1.CustomResourceDefinition:
// 		if err := v.Get(); err != nil {
// 			return false, err
// 		}
// 		crd := &apiextv1beta1.CustomResourceDefinition{}
// 		if err := scheme.Scheme.Convert(v.Object, crd, nil); err != nil {
// 			return false, err
// 		}
// 		if !w.crdBetaReady(*crd) {
// 			return false, nil
// 		}
// 	case *apiextv1.CustomResourceDefinition:
// 		if err := v.Get(); err != nil {
// 			return false, err
// 		}
// 		crd := &apiextv1.CustomResourceDefinition{}
// 		if err := scheme.Scheme.Convert(v.Object, crd, nil); err != nil {
// 			return false, err
// 		}
// 		if !w.crdReady(*crd) {
// 			return false, nil
// 		}
// 	case *appsv1.StatefulSet, *appsv1beta1.StatefulSet, *appsv1beta2.StatefulSet:
// 		sts, err := w.c.AppsV1().StatefulSets(v.Namespace).Get(context.Background(), v.Name, metav1.GetOptions{})
// 		if err != nil {
// 			return false, err
// 		}
// 		if !w.statefulSetReady(sts) {
// 			return false, nil
// 		}
// 	case *corev1.ReplicationController, *extensionsv1beta1.ReplicaSet, *appsv1beta2.ReplicaSet, *appsv1.ReplicaSet:
// 		ok, err = w.podsReadyForObject(v.Namespace, value)
// 	}
// 	return false, nil
// }
