/*
Copyright 2021 The Kubernetes Authors.

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

package conditions

import (
	"context"
	"log"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimachinerywait "k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
)

type Condition struct {
	resources *resources.Resources

	// verbose is used to enable additional logs from the Wait function that is getting executed.
	// This can help in debugging long-running tests.
	verbose bool
}

// New is used to create a new Condition that can be used to perform a series of pre-defined wait checks
// against a resource in question
func New(r *resources.Resources) *Condition {
	return &Condition{resources: r}
}

// WithVerboseLog is used to enable additional verbose logs from the Wait condition to help debug tests
func (c *Condition) WithVerboseLog() *Condition {
	c.verbose = true
	return c
}

// log is used to enable additional logs conditionally based on c.verbose configuration
func (c *Condition) log(message string, args ...interface{}) {
	if c.verbose {
		log.Printf(message, args...)
	}
}

// ResourceScaled is a helper function used to check if the resource under question has a pre-defined number of
// replicas. This can be leveraged for checking cases such as scaling up and down a deployment or STS and any
// other scalable resources.
func (c *Condition) ResourceScaled(obj k8s.Object, scaleFetcher func(object k8s.Object) int32, replica int32) apimachinerywait.ConditionFunc {
	return func() (done bool, err error) {
		c.log("Checking if the resource %s/%s has been scaled to %d", obj.GetNamespace(), obj.GetName(), replica)
		if err := c.resources.Get(context.TODO(), obj.GetName(), obj.GetNamespace(), obj); err != nil {
			return false, nil
		}
		return scaleFetcher(obj) == replica, nil
	}
}

// ResourceDeleted is a helper function used to check if a resource under question has been deleted. This will enable
// testing cases where the resource have a finalizer and the DELETE operation of such resource have been triggered and
// you want to wait until the resource has been deleted.
//
// This method can be leveraged against any Kubernetes resource to check the deletion workflow and it does so by
// checking the resource and waiting until it obtains a v1.StatusReasonNotFound error from the API
func (c *Condition) ResourceDeleted(obj k8s.Object) apimachinerywait.ConditionFunc {
	return func() (done bool, err error) {
		c.log("Checking for Resource deletion of %s/%s", obj.GetNamespace(), obj.GetName())
		if err := c.resources.Get(context.Background(), obj.GetName(), obj.GetNamespace(), obj); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}
}

// JobConditionMatch is a helper function that can be used to check the Job Completion or runtime status against a
// specific condition. This function accepts both conditionType and conditionState as argument and hence you can use this
// to match both positive or negative cases with suitable values passed to the arguments.
func (c *Condition) JobConditionMatch(job k8s.Object, conditionType batchv1.JobConditionType, conditionState v1.ConditionStatus) apimachinerywait.ConditionFunc {
	return func() (done bool, err error) {
		c.log("Checking for Job Condition %s/%s on %s/%s", conditionType, conditionState, job.GetNamespace(), job.GetName())
		if err := c.resources.Get(context.TODO(), job.GetName(), job.GetNamespace(), job); err != nil {
			return false, err
		}
		for _, cond := range job.(*batchv1.Job).Status.Conditions {
			if cond.Type == conditionType && cond.Status == conditionState {
				done = true
			}
		}
		return
	}
}

// PodConditionMatch is a helper function that can be used to check a specific condition match for the Pod in question.
// This is extended into a few simplified match helpers such as PodReady and ContainersReady as well.
func (c *Condition) PodConditionMatch(pod k8s.Object, conditionType v1.PodConditionType, conditionState v1.ConditionStatus) apimachinerywait.ConditionFunc {
	return func() (done bool, err error) {
		c.log("Checking for Pod Condition %s/%s on %s/%s", conditionType, conditionState, pod.GetNamespace(), pod.GetName())
		if err := c.resources.Get(context.TODO(), pod.GetName(), pod.GetNamespace(), pod); err != nil {
			return false, err
		}
		for _, cond := range pod.(*v1.Pod).Status.Conditions {
			if cond.Type == conditionType && cond.Status == conditionState {
				done = true
			}
		}
		return
	}
}

// PodPhaseMatch is a helper function that is used to check and see if the Pod Has reached a specific Phase of the
// runtime. This can be combined with PodConditionMatch to check if a specific condition and phase has been met.
// This will enable validation such as checking against CLB of a POD.
func (c *Condition) PodPhaseMatch(pod k8s.Object, phase v1.PodPhase) apimachinerywait.ConditionFunc {
	return func() (done bool, err error) {
		c.log("Checking for Pod %v Condition of %s/%s", phase, pod.GetNamespace(), pod.GetName())
		if err := c.resources.Get(context.Background(), pod.GetName(), pod.GetNamespace(), pod); err != nil {
			return false, err
		}
		return pod.(*v1.Pod).Status.Phase == phase, nil
	}
}

// PodReady is a helper function used to check if the pod condition v1.PodReady has reached v1.ConditionTrue state
func (c *Condition) PodReady(pod k8s.Object) apimachinerywait.ConditionFunc {
	return c.PodConditionMatch(pod, v1.PodReady, v1.ConditionTrue)
}

// ContainersReady is a helper function used to check if the pod condition v1.ContainersReady has reached v1.ConditionTrue
func (c *Condition) ContainersReady(pod k8s.Object) apimachinerywait.ConditionFunc {
	return c.PodConditionMatch(pod, v1.ContainersReady, v1.ConditionTrue)
}

// PodRunning is a helper function used to check if the pod.Status.Phase attribute of the Pod has reached v1.PodRunning
func (c *Condition) PodRunning(pod k8s.Object) apimachinerywait.ConditionFunc {
	return c.PodPhaseMatch(pod, v1.PodRunning)
}

// JobCompleted is a helper function used to check if the Job has been completed successfully by checking if the
// batchv1.JobCompleted has reached the v1.ConditionTrue state
func (c *Condition) JobCompleted(job k8s.Object) apimachinerywait.ConditionFunc {
	return c.JobConditionMatch(job, batchv1.JobComplete, v1.ConditionTrue)
}

// JobFailed is a helper function used to check if the Job has failed by checking if the batchv1.JobFailed has reached
// v1.ConditionTrue state
func (c *Condition) JobFailed(job k8s.Object) apimachinerywait.ConditionFunc {
	return c.JobConditionMatch(job, batchv1.JobFailed, v1.ConditionTrue)
}
