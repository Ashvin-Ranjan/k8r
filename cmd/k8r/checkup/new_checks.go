// Description: This file contains code for problems related to pods

package checkup

import (
	"context"
	"fmt"

	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ProblemMaxedOutHPAs when HPAs for a cluster are maxed out
// https://github.com/Ashvin-Ranjan/k8r/wiki/MaxedOutHPAs
var ProblemMaxedOutHPAs = Problem{
	ID:               "MaxedOutHPAs",
	ShortDescription: "A pod's HPAs current replicas is equal to its max",
	HelpURL:          "https://github.com/Ashvin-Ranjan/k8r/wiki/MaxedOutHPAs",
	Detector: func(ctx context.Context, obj runtime.Object, _ *Config) (string, bool, bool) {
		// Since this is an HPA issue we can assume what is passed in is an HPA
		hpa, ok := obj.(*v1.HorizontalPodAutoscaler)
		if !ok {
			return "", false, false
		}

		// If the max replicas allowed is equal to the current replicas, the HPA is considered maxed out
		if hpa.Spec.MaxReplicas == hpa.Status.CurrentReplicas {
			return fmt.Sprintf("%s has %d/%d replicas", hpa.Name, hpa.Status.CurrentReplicas, hpa.Spec.MaxReplicas), true, true
		}

		return "", false, false
	},
}

// ProblemHighRestarts is a problem with a cluster that keeps on restarting
// https://github.com/Ashvin-Ranjan/k8r/wiki/HighRestarts
var ProblemHighRestarts = Problem{
	ID:               "HighRestarts",
	ShortDescription: "A pod keeps restarting which can indicate a problem",
	HelpURL:          "https://github.com/Ashvin-Ranjan/k8r/wiki/HighRestarts",
	Detector: func(ctx context.Context, obj runtime.Object, cfg *Config) (string, bool, bool) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return "", false, false
		}

		// We don't check if the pod is online or not because if
		// it is constantly crashing it may be offline for long
		// periods of time

		// Check if the pod has any containers that have crash counts above the threshold
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if cs.RestartCount >= int32(cfg.RestartThreshold) {
				return fmt.Sprintf("Container %s has restarted %d time(s)", pod.Name, cs.RestartCount), true, true
			}
		}

		return "", false, false
	},
}
