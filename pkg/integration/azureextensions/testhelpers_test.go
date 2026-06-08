/*
Copyright 2024 The K8sGPT Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azureextensions

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// makePod creates a test pod with configurable failure states.
func makePod(name, namespace string, crashLoop bool, imagePullErr bool, restartCount int32) v1.Pod {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "main", Image: "test:latest"},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "main",
					RestartCount: restartCount,
					Ready:        true,
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	if crashLoop {
		pod.Status.ContainerStatuses[0].Ready = false
		pod.Status.ContainerStatuses[0].State = v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{
				Reason:  "CrashLoopBackOff",
				Message: "back-off 5m0s restarting failed container",
			},
		}
	}

	if imagePullErr {
		pod.Status.ContainerStatuses[0].Ready = false
		pod.Status.ContainerStatuses[0].State = v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{
				Reason:  "ImagePullBackOff",
				Message: "Back-off pulling image \"mcr.microsoft.com/test:latest\"",
			},
		}
	}

	return pod
}
