# csi-driver-rclone

rclone csi driver, packaged for k8s

![Version: v1.0.0](https://img.shields.io/badge/Version-v1.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v1.0.0](https://img.shields.io/badge/AppVersion-v1.0.0-informational?style=flat-square)

## Installing the Chart

## Config PVC

`rclone` likes to manage it's own config. This chart requires a ReadWriteMany PVC
to store this config and make it available to all instance of the DaemonSet.

An example PVC and Pod are included with this chart:

```bash
kubectl -n kube-system apply -f rclone-config.yaml
kubectl -n kube-system exec --tty rclone-config
# use this shell session to write your rclone config, then delete the pod
kubectl -n kube-system delete pod rclone-config
```

## Helm Chart

```bash
helm repo add csi-driver-rclone https://raw.githubusercontent.com/cornfeedhobo/csi-driver-rclone/master/deploy/helm/charts
helm template csi-driver-rclone/csi-driver-rclone \
  --set "containers.driver.remote=myrcloneremote:/k8s/basepath"
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| global.labels | object | `{}` | Additional labels. |
| global.annotations | object | `{}` | Additional annotations. |
| csi.driverName | string | `"rclone.csi.k8s.io"` | The name of the driver. |
| csi.storageClassName | string | `"rclone"` | The name for the created StorageClass resource, allowing this driver to handle dynamic provisioning. |
| pvc.name | string | `"rclone-config"` | This PVC can be created manually before chart installation. Always required. |
| pvc.create | bool | `false` | Create the PVC as part of this helm chart. Warning, this also means the volume will be deleted when this chart is uninstalled. Note, you can populate the volume using a temporary pod with the created PVC mounted. |
| pvc.labels | object | `{}` | Additional labels. |
| pvc.annotations | object | `{}` | Additional annotations. |
| pvc.storageClass | string | `""` | The storage class backing the shared rclone pvc. Required if creating. Must support ReadWriteMany, e.g. configStorageClass: "nfs". DO NOT set to 'rclone'. |
| daemonset.labels | object | `{}` | Additional labels. |
| daemonset.annotations | object | `{}` | Additional annotations. |
| daemonset.updateStrategy | object | `{"rollingUpdate":{"maxUnavailable":1},"type":"RollingUpdate"}` | Daemonset update strategy. |
| daemonset.nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node selector. |
| daemonset.pod.labels | object | `{}` | Additional labels. |
| daemonset.pod.annotations | object | `{}` | Additional annotations. |
| deployment.labels | object | `{}` | Additional labels. |
| deployment.annotations | object | `{}` | Additional annotations. |
| deployment.nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node selector. |
| deployment.replicas | int | `1` | Replica count. The driver supports leadership elections, meaning multiple controllers should work, but remains untested. |
| deployment.pod.labels | object | `{}` | Additional labels. |
| deployment.pod.annotations | object | `{}` | Additional annotations. |
| containers.rclone.image.repo | string | `"bitnami/rclone"` |  |
| containers.rclone.image.tag | string | `"latest"` |  |
| containers.rclone.image.pullPolicy | string | `"IfNotPresent"` |  |
| containers.rclone.resources | object | `{}` |  |
| containers.rclone.verbosity | int | `1` |  |
| containers.driver.image.repo | string | `"ghcr.io/cornfeedhobo/csi-driver-rclone"` |  |
| containers.driver.image.tag | string | `""` |  |
| containers.driver.image.pullPolicy | string | `"IfNotPresent"` |  |
| containers.driver.resources.limits.memory | string | `"300Mi"` |  |
| containers.driver.resources.requests.cpu | string | `"10m"` |  |
| containers.driver.resources.requests.memory | string | `"20Mi"` |  |
| containers.driver.remote | string | `""` |  |
| containers.driver.verbosity | int | `1` |  |
| containers.driver.args | list | `[]` |  |
| containers.provisioner.image.repo | string | `"registry.k8s.io/sig-storage/csi-provisioner"` |  |
| containers.provisioner.image.tag | string | `"v4.0.0"` |  |
| containers.provisioner.image.pullPolicy | string | `"IfNotPresent"` |  |
| containers.provisioner.resources.limits.memory | string | `"400Mi"` |  |
| containers.provisioner.resources.requests.cpu | string | `"10m"` |  |
| containers.provisioner.resources.requests.memory | string | `"20Mi"` |  |
| containers.provisioner.verbosity | int | `1` |  |
| containers.liveness.image.repo | string | `"registry.k8s.io/sig-storage/livenessprobe"` |  |
| containers.liveness.image.tag | string | `"v2.12.0"` |  |
| containers.liveness.image.pullPolicy | string | `"IfNotPresent"` |  |
| containers.liveness.resources.limits.memory | string | `"100Mi"` |  |
| containers.liveness.resources.requests.cpu | string | `"10m"` |  |
| containers.liveness.resources.requests.memory | string | `"20Mi"` |  |
| containers.liveness.verbosity | int | `1` |  |
| containers.registrar.image.repo | string | `"registry.k8s.io/sig-storage/csi-node-driver-registrar"` |  |
| containers.registrar.image.tag | string | `"v2.10.0"` |  |
| containers.registrar.image.pullPolicy | string | `"IfNotPresent"` |  |
| containers.registrar.resources.limits.memory | string | `"100Mi"` |  |
| containers.registrar.resources.requests.cpu | string | `"10m"` |  |
| containers.registrar.resources.requests.memory | string | `"20Mi"` |  |
| containers.registrar.verbosity | int | `1` |  |

---

Generated with [`helm-docs -s file`](https://github.com/norwoodj/helm-docs)
