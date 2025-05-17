# sustain-kube

## Official Guide

### Prerequisites

- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Setup the cluster

**Automatically setup with vagrant**

Make sure your local computer has installed Vagrant.
If not, come to the [page](https://developer.hashicorp.com/vagrant/downloads) to find one that matches with your OS.

```bash
vagrant plugin install vagrant-hostmanager
vagrant plugin install vagrant-vmware-desktop
```

**Install required dependencies**
```bash
sudo apt install make

# Go Wiki (Ubuntu): https://go.dev/wiki/Ubuntu
sudo add-apt-repository ppa:longsleep/golang-backports
sudo apt update
sudo apt install golang-go
```

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

(Skip the part if you're not developer/owner)

```sh
make docker-build docker-push IMG=nu1lspaxe/sustain-kube:latest
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
# This operator will expose the custom metrics to Prometheus.
# For simplicity, we just use prometheus-operator in this case.
# Prometheus Operator: https://prometheus-operator.dev/docs/getting-started/installation/
git clone https://github.com/prometheus-operator/kube-prometheus.git
cd kube-prometheus
kubectl create -f manifests/setup -f manifests

# Clone the target operator repository
git clone https://github.com/nu1lspaxe/sustain-kube.git
cd sustain-kube
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=nu1lspaxe/sustain-kube:latest
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
> privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

> **NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/sustain-kube:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f `<URL for YAML BUNDLE>` to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/sustain-kube/<tag or branch>/dist/install.yaml
```

---

## Dev Guide

### To-do List

- [X] Define `CarbonEstimator` Spec
- [X] Retrieve data from Prometheus
- [X] Calculate power consumption
- [X] (Manual) Set up cluster (master node + 2 worker nodes)
- [X] (Auto) Set up cluster (master node + 2 worker nodes)
- [ ] Calculate carbon emission with Electricity Maps
- [X] Expose result metrics to Prometheus
- [ ] Build Grafana dashboard

### Prerequisite

```bash
# Initialize the project
kubebuilder init --domain sustain-kube.com --repo sustain_kube
# Create api
kubebuilder create api --version v1alpha1 --kind CarbonEstimator
```

### Operator & Custom Resource

- Expose services for local testing
  ```bash
  kubectl port-forward service/prometheus-k8s -n monitoring 9090:9090 --address 0.0.0.0 &
  ```
- Check controller state
  ```bash
  kubectl describe carbonestimator carbonestimator-sample
  ```

#### Custom Resource

```yaml
# config/samples/_v1alpha1_carbonestimator.yaml

apiVersion: sustain-kube.com/v1alpha1
kind: CarbonEstimator
metadata:
  labels:
    app.kubernetes.io/name: sustain-kube
    app.kubernetes.io/managed-by: kustomize
  name: carbonestimator-sample
spec:
  prometheusURL: http://prometheus-k8s.monitoring.svc.cluster.local:9090 # (replace)
  levelCritical: 10 # (replace)
  levelWarning: 5 # (replace)
  powerConsumptionCPU: '15' # power draw for cores 
  powerConsumptionMemory: '1.5' # power draw for memory
```

### Monitoring & Testing

#### Check DNS Connection

1. Create test Pod

   ```bash
   kubectl apply -f config/samples/alpine.yml
   ```

   ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: alpine
      namespace: default
    spec:
      containers:
      - image: alpine:latest
        command:
          - sleep
          - "3600"
        imagePullPolicy: IfNotPresent
        name: alpine
      restartPolicy: Always
   ```
2. Test connection and service healthy

   ```bash
   kubectl exec -it alpine -- apk --update add curl
   # kubectl exec -it alpine -- nslookup prometheus-kube-prometheus-prometheus.default.svc.cluster.local

   kubectl exec -it alpine -- apk --update add net-tools
   # kubectl exec -it alpine -- curl -X GET http://prometheus-kube-prometheus-prometheus.default.svc.cluster.local:9090/-/healthy
   ```

#### Top CPU & Memory Usages

1. `kubectl top <pod|node>`
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
   ```
