I want you to analyze a given prowjob and make sure it is following best practices.


There is a script called config/mkpj.sh you can invoke with the --job flag to generate a prowjob.

Once its generated, you will evaluate the following:

1. pod_spec should have exactly 1 container
1. If the prowjob has the label `preset-dind-enabled = "true"`, I want you read its logs using log-analyzer.md and verify if Docker is being used in the logs.
1. If the prowjob has the label `preset-service-account = true`, I want you to read its logs using log-analyzer.md and verify if the job is creating Google Cloud Infrastructure.
1. The image specified in pod_spec.containers[0].image has the following requirements:
   1. It must be this image: `us-central1-docker.pkg.dev/k8s-staging-test-infra/images/kubekins-e2e` 
   1. A prowjob can use this image `gcr.io/k8s-staging-test-infra/kubekins-e2e` if the program `/workspace/scenarios/kubernetes_e2e.py` is ran
   1. The repositories k8s.io and release may use non-standard images
1. containers using these images must always have exactly a single command called runner.sh
   1. `us-central1-docker.pkg.dev/k8s-staging-test-infra/images/kubekins-e2e`
   1. `gcr.io/k8s-staging-test-infra/kubekins-e2e`
