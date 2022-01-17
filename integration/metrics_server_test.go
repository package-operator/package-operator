package integration_test

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) TestMetricsServer() {
	ctx := context.Background()
	pod := pod_metricsClient()

	// create the metrics client pod
	err := integration.Client.Create(ctx, pod)
	s.Require().NoError(err)

	// wait until Pod is available
	err = integration.WaitForObject(
		s.T(), defaultPodAvailabilityTimeout, pod, "to be Ready",
		func(obj client.Object) (done bool, err error) {
			p := obj.(*corev1.Pod)
			for _, podCondition := range p.Status.Conditions {
				if podCondition.Type == corev1.PodReady && podCondition.Status == corev1.ConditionTrue {
					return true, nil
				}
			}
			return false, nil
		})
	s.Require().NoError(err)

	s.Run("test_https_endpoint", func() {
		httpsMetricsAddr := "https://addon-operator-metrics.addon-operator.svc:8080/healthz"
		caCertPath := pod.Spec.Containers[0].VolumeMounts[0].MountPath + "ca.crt"

		command := []string{"curl", "--cacert", caCertPath, httpsMetricsAddr}

		httpsCurlCallResult, _, err := integration.ExecCommandInPod(pod.Namespace, pod.Name, pod.Spec.Containers[0].Name, command)
		s.Require().NoError(err)
		s.Assert().Equal("404 page not found", httpsCurlCallResult)
	})

	s.Run("test_http_endpoint", func() {
		httpMetricsAddr := "http://addon-operator-metrics.addon-operator.svc:8083/healthz"

		command := []string{"curl", httpMetricsAddr}

		httpCurlCallResult, _, err := integration.ExecCommandInPod(pod.Namespace, pod.Name, pod.Spec.Containers[0].Name, command)
		s.Require().NoError(err)
		s.Assert().Equal("404 page not found", httpCurlCallResult)
	})

	s.T().Cleanup(func() {
		s.T().Logf("waiting for pod %s/%s to be deleted...", pod.Namespace, pod.Name)

		err := integration.Client.Delete(ctx, pod, client.PropagationPolicy("Foreground"))
		s.Require().NoError(client.IgnoreNotFound(err), "delete Pod: %v", pod)

		// wait until Pod is gone
		err = integration.WaitToBeGone(s.T(), defaultPodDeletionTimeout, pod)
		s.Require().NoError(err, "wait for Pod to be deleted")
	})
}
