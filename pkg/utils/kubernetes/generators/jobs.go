package generators

import (
	"bytes"
	"fmt"

	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// DefaultRequiredCompletionsForJobs indicates the number of Pod
	// completions that will be required by default for any generated
	// Job resources.
	DefaultRequiredCompletionsForJobs int32 = 1
)

// GenerateBashJob will create a ConfigMap and a Job that can be deployed to run
// a bash script given the container image you want the job to run and the shell
// commands you want the job to execute as arguments. By default this will only
// generate a single container, and you can provide env vars to be loaded into it.
func GenerateBashJob(image, imageTag string, commands ...string) (*corev1.ConfigMap, *batchv1.Job) {
	// build the job script
	script := new(bytes.Buffer)
	script.WriteString("#!/bin/bash\n")
	script.WriteString("set -euox pipefail\n")
	for _, command := range commands {
		script.WriteString(fmt.Sprintf("%s\n", command))
	}

	// configure the script in a configmap for mounting
	jobName := uuid.New().String()
	cfgmap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Data: map[string]string{
			jobName: script.String(),
		},
	}

	// build the job itself
	mountPath := fmt.Sprintf("/opt/%s", jobName)
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    jobName,
						Image:   fmt.Sprintf("%s:%s", image, imageTag),
						Command: []string{"/bin/bash", mountPath},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      jobName,
							MountPath: mountPath,
							SubPath:   jobName,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: jobName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: jobName},
							},
						},
					}},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
			Completions: &DefaultRequiredCompletionsForJobs,
		},
	}

	return &cfgmap, &job
}
