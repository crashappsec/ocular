# Ocular Quickstart Guide

Ocular can be deployed on any Kubernetes cluster that supports Custom Resource Definitions (CRDs) and
Resource Controllers. The following instructions will guide you through the setup process.

## Prerequisites
- A Kubernetes v1.28.0+ cluster.
- An ability to create resources in the cluster (e.g., admin access).
- [OPTIONAL] `kubectl` command-line tool installed and configured to interact with your cluster.
- [OPTIONAL] `helm` command-line tool installed if you prefer Helm for installation.
- [OPTIONAL] `jq` command-line tool installed for JSON processing.
- [OPTIONAL] Access to a container registry if you plan to use custom images.

## Installation Steps
1. **Install Ocular into the cluster:**
    
    Ocular can either be installed via helm or by applying the manifests directly using a tool like
    `kubectl`. Choose one of the following methods:
    - **Using Helm:**
      ```sh
      helm repo add crashoverride https://crashappsec.github.io/helm-charts
      helm install ocular crashoverride/ocular \
        --namespace ocular-system \
        --create-namespace
      ```
    - **Using kubectl:**
    
      ```sh
      # Install the latest verion of ocular
      curl -s https://api.github.com/repos/crashappsec/ocular/releases/latest \
      | grep "browser_download_url.*yaml" \
      | cut -d : -f 2,3 \
      | tr -d \" \
      | wget -qi - -O - \
      | kubectl apply -f -
            
      # OR
      # Install a specific ocular version vX.Y.Z
      OCULAR_VERSION="vX.Y.Z" # replace with desired version
      curl -s https://api.github.com/repos/crashappsec/ocular/releases/download/${OCULAR_VERSION}/ocular.yaml
      ```
2. **Install ocular default integrations into the cluster:**
   **NOTE**: this step is technically optional, but the remainder of this quickstart guide
    assumes that you have installed the default integrations. If you choose to skip this step,
    you will need to create your own integrations to perform scans.

   Ocular is an orchestration platform that relies on integrations to perform specific tasks.
   These tasks include downloading scan targets, running scanners, uploading result files and
   crawling for new scan targets. The additional package `ocular-default-integrations` 
   provides a set of default integrations that can be used out-of-the-box to perform 
   scans using popular tools and services. Just like before there are two methods to install
   the default integrations, either via helm or by applying the manifests directly using a tool like
   `kubectl`. Choose one of the following methods:
    - **Using Helm:**
      ```sh
      # Make sure to add the repo if not done already
      # helm repo add crashoverride https://crashappsec.github.io/helm-charts
      
      # This should be the namespace you want to perform scan in.
      # It should probably be different than the 'ocular-system' namespace
      # where the ocular controller runs.
      export NAMESPACE=default
    
      helm install ocular-default-integrations \
      crashoverride/ocular-default-integrations \
      --namespace $NAMESPACE
      ```
    - **Using kubectl:**

      ```sh
      # This should be the namespace you want to perform scan in.
      # It should probably be different than the 'ocular-system' namespace
      # where the ocular controller runs.
      export NAMESPACE=default
      # Install the latest verion of ocular
      curl -s https://api.github.com/repos/crashappsec/ocular-default-integrations/releases/latest \
      | grep "browser_download_url.*yaml" \
      | cut -d : -f 2,3 \
      | tr -d \" \
      | wget -qi - -O - \
      | kubectl apply -n "${NAMESPACE}" -f -
            
      # OR
      # Install a specific ocular version vX.Y.Z
      OCULAR_DEFAULTS_VERSION="vX.Y.Z" # replace with desired version
      curl -s https://api.github.com/repos/crashappsec/ocular-default-integrations/releases/download/${OCULAR_DEFAULTS_VERSION}/ocular-default-integrations.yaml
      ```
3. **Verify the installation:**
    After installation, verify that the Ocular components are running correctly:
    ```sh
    kubectl get pods -n ocular-system
    ```
    You should see the Ocular controller pod in a `Running` state.
    [OPTIONAL] verify that the default integrations have been configured correctly:
    ```sh
    kubectl get downloaders -n <NAMESPACE>
    ```
    You should see a list of downloader integrations each with `ocular-defaults-` prefix.
4. **Run a pipeline to scan a git repository**
   In order to run a pipeline we first need to create a `Profile` resource that defines
   the scanners to use and their configurations, and a `Downloader` resource that defines
   how to fetch the scan targets. In this quickstart guide we will be scanning a git repository,
   so we can use the `ocular-defaults-git` downloader provided by the default integrations package.
   (if you skipped installing the default integrations you will need to create your own downloader).
   
    Create a file named `quickstart-profile.yaml` with the following content:
    ```yaml
    apiVersion: ocular.crashoverride.run/v1beta1
    kind: Profile
    metadata:
      name: quickstart
      namespace: default # This should be the same namespace where you installed the default integrations
    spec:
      containers:
      - name: "semgrep"
        image: "semgrep/semgrep:latest"
        imagePullPolicy: "IfNotPresent"
        # OCULAR_RESULTS_DIR is an environment variable containing the name
        # of the folder that artifacts should be collected from.
        # See 'Artifacts' section of the manual for more info
        command: ["/bin/sh", "-c"]
        args: ["semgrep scan --json --config=auto . | tee $OCULAR_RESULTS_DIR/semgrep.json"]
      - name: "trufflehog"
        image: "trufflesecurity/trufflehog:latest"
        imagePullPolicy: "IfNotPresent"
        command: ["/bin/sh", "-c"]
        args: ["trufflehog git --json --no-update file://. | tee $OCULAR_RESULTS_DIR/trufflehog.jsonl"]
      artifacts:
      - semgrep.json
      - trufflehog.jsonl
    ```
    This profile defines two scanners:`semgrep` and `trufflehog`, each running in its own container.
    Each scanner starts in the same directory that the scan targets are downloaded to by the downloader.
    The results of each scanner are saved to files specified in the `artifacts` section. While this profile doesn't
    define any uploaders to send the results to a remote service, the results files are still left in the
    artifacts directory and can be accessed later.

    Apply the profile to the cluster:
    ```sh
    kubectl apply -f quickstart-profile.yaml
    ```
   
    Finally, create a file named `quickstart-pipeline.yaml` with the following content:
    ```yaml
    apiVersion: ocular.crashoverride.run/v1beta1
    kind: Pipeline
    metadata:
        # better to use 'generateName' so that multiple pipelines
        # can be created from the same spec without name conflicts
        generateName: quickstart-
        namespace: default
    spec:
        downloaderRef:
            name: ocular-defaults-git
        profileRef:
            name: quickstart
        target:
            identifier: "https://github.com/crashappsec/ocular"
    ```
    This pipeline uses the `ocular-defaults-git` downloader to fetch the scan targets from
    the specified git repository, and the `quickstart` profile to define the scanners to run.
   Apply the pipeline to the cluster:
    ```sh
    kubectl create -f quickstart-pipeline.yaml
    # Name of the created pipeline will be returned
    ```
5. **Monitor the pipeline execution:**
    You can monitor the status of the pipeline either by describing the pipeline resource and viewing the `Status` section:
    ```sh
    kubectl describe pipeline ${PIPELINE_NAME} -n default
    ```
    Or by viewing the pods created for the pipeline:
    ```sh
    kubectl get pods -n default -l ocular.crashoverride.run/pipeline=${PIPELINE_NAME}
    ```
   
   Since there are no uploaders defined in the profile, the results files are printed to the logs of the scanner pods.
   You can view the logs of each scanner pod to see the results:
   ```sh
   kubectl logs -l ocular.crashoverride.run/pipeline=${PIPELINE_NAME} -n default --all-containers
   ```


