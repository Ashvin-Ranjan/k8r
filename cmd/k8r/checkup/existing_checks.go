// Taken from: https://github.com/getoutreach/devenv/blob/main/cmd/devenv/debug/problem_pods.go
// Rights given from Outreach under Apache License 2.0

// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains code for problems related to pods

// EDIT: All Detector functions have had their method signatures changed

package checkup

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ProblemPodCrashLoopBackOff is a problem with a pod that is in a crash loop
// https://github.com/getoutreach/devenv/wiki/PodCrashLoopBackOff
var ProblemPodCrashLoopBackOff = Problem{
	ID:               "PodCrashLoopBackOff",
	ShortDescription: "A pod is in a crash loop backoff state, meaning it is crashing repeatedly",
	Detector: func(ctx context.Context, obj runtime.Object, _ *Config) (string, bool, bool) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return "", false, false
		}

		isCrashLoopBackoff := func(cs *corev1.ContainerStatus) bool {
			return cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff"
		}

		// Check if the pod has any containers that are in a crash loop
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if isCrashLoopBackoff(cs) {
				return fmt.Sprintf("Container %s in a crash loop backoff state: %v",
					cs.Name, cs.LastTerminationState.Terminated.Message,
				), false, true
			}
		}

		// Check the init containers
		for i := range pod.Status.InitContainerStatuses {
			cs := &pod.Status.InitContainerStatuses[i]
			if isCrashLoopBackoff(cs) {
				return fmt.Sprintf("Init container %s in a crash loop backoff state: %v",
					cs.Name, cs.LastTerminationState.Terminated.Message,
				), false, true
			}
		}

		return "", false, false
	},
}

// ProblemPodNotReady is a problem with a pod that is not ready
// https://github.com/getoutreach/devenv/wiki/PodNotReady
var ProblemPodNotReady = Problem{
	ID:               "PodNotReady",
	ShortDescription: "A pod is not ready which can indicate a problem with the pod",
	Detector: func(ctx context.Context, obj runtime.Object, _ *Config) (string, bool, bool) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return "", false, false
		}

		// We don't care about pods that are not running
		// e.g. jobs. Those will have their own problems that
		// we can detect.
		if pod.Status.Phase != corev1.PodRunning {
			return "", false, false
		}

		// Check if the pod has any containers that are not ready
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if !cs.Ready {
				return fmt.Sprintf("Container %s is not ready", cs.Name), false, true
			}
		}

		return "", false, false
	},
}

// ProblemPodImagePullBackOff is a problem with a pod that is
// in a image pull backoff state
// https://github.com/getoutreach/devenv/wiki/PodImagePullBackOff
var ProblemPodImagePullBackOff = Problem{
	ID:               "PodImagePullBackOff",
	ShortDescription: "A pod is in a image pull backoff state, meaning it is unable to pull the image",
	Detector: func(ctx context.Context, obj runtime.Object, _ *Config) (string, bool, bool) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return "", false, false
		}

		// isImagePullBackOff checks if the container is in a image pull backoff state
		isImagePullBackOff := func(cs *corev1.ContainerStatus) bool {
			// Handle both ImagePullBackOff and ErrImagePull because one is a backoff and one
			// is the current condition. They are essentially the same thing.
			return cs.State.Waiting != nil &&
				(cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull")
		}

		// getImageForContainerStatus returns the image for a container status
		getImageForContainerStatus := func(isInitContainer bool, cs *corev1.ContainerStatus) string {
			sl := pod.Spec.Containers
			if isInitContainer {
				sl = pod.Spec.InitContainers
			}

			var container *corev1.Container
			for i := range sl {
				c := &sl[i]
				if c.Name == cs.Name {
					container = c
					break
				}
			}
			if container == nil {
				return "unknown"
			}

			return container.Image
		}

		// Check if the pod has any containers that are in a image pull backoff state
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			imageName := getImageForContainerStatus(false, cs)
			if isImagePullBackOff(cs) {
				return fmt.Sprintf("Container %s is failing to pull its image (%s)", cs.Name, imageName), false, true
			}
		}

		// Check the init containers
		for i := range pod.Status.InitContainerStatuses {
			cs := &pod.Status.InitContainerStatuses[i]
			imageName := getImageForContainerStatus(true, cs)
			if isImagePullBackOff(cs) {
				return fmt.Sprintf("Container %s is failing to pull its image (%s)", cs.Name, imageName), false, true
			}
		}

		return "", false, false
	},
}

// ProblemPodOOMKilled is a problem with a pod that is/was OOM killed
// https://github.com/getoutreach/devenv/wiki/PodOOMKilled
var ProblemPodOOMKilled = Problem{
	ID:               "PodOOMKilled",
	ShortDescription: "A pod was killed because it ran out of memory recently",
	Detector: func(ctx context.Context, obj runtime.Object, _ *Config) (string, bool, bool) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return "", false, false
		}

		// Check if the pod has any containers that were OOM killed recently
		// or are currently OOM killed
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
				return fmt.Sprintf("Container %s was killed because it ran out of memory", cs.Name), false, true
			}

			// Check the last termination state as well
			if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return fmt.Sprintf("Container %s was recently killed because it ran out of memory: %s",
					cs.Name,
					cs.LastTerminationState.Terminated.FinishedAt.Time,
				), true, true
			}
		}

		return "", false, false
	},
}

// PodPending is a problem with a pod that is stuck pending
// https://github.com/getoutreach/devenv/wiki/PodPending
var ProblemPodPending = Problem{
	ID:               "PodPending",
	ShortDescription: "A pod is pending",
	Detector: func(ctx context.Context, obj runtime.Object, _ *Config) (string, bool, bool) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return "", false, false
		}

		// We don't care about pods that are not pending
		if pod.Status.Phase != corev1.PodPending {
			return "", false, false
		}

		// Check if the pod has any containers that are not ready
		for i := range pod.Status.ContainerStatuses {
			cs := &pod.Status.ContainerStatuses[i]
			if cs.State.Waiting != nil {
				return fmt.Sprintf("Container %s is pending: %s", cs.Name, cs.State.Waiting.Message), false, true
			}
		}

		// Check the init containers
		for i := range pod.Status.InitContainerStatuses {
			cs := &pod.Status.InitContainerStatuses[i]
			if cs.State.Waiting != nil {
				return fmt.Sprintf("Init container %s is pending: %s", cs.Name, cs.State.Waiting.Message), false, true
			}
		}

		return "", false, false
	},
}
