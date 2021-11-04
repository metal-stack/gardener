# Kubernetes versions for worker pools

Since Gardener **`1.36.x`** worker pools can have different kubernetes versions specified than the control plane.

It must be enabled by setting the featureGate: `enableWorkerPoolKubernetesVersion: true` in the gardenlet.

Before, all worker pools inherited the kubernetes version of the control plane. Once the kubernetes version of the control plane was modified, all worker pools have been updated as well. Either by rolling the nodes in case of a minor kubernetes change, or in place for patch version updates.

Some workloads, especially dealing with lots of data, wanted to be able to gradually update the kubernetes version, first the control plane, then provide new workers with the new version set, but keep existing worker. Then reschedule the pods to the new workers.

## Constraints

```yaml
spec:
    kubernetes:
        version: 1.20.1
    provider:
        workers:
            - name: data1
              kubernetes:
                version: 1.19.1
            - name: data2
```

- If the version is not specified in a worker pool, the kubernetes version of the kubelet is inherited from the control plane
- If the version is specified, it must meet the following constraints:
  - equal to the control plane version specified
  - up to two minor versions lower than the control plane version
  - if it was not specified before, no downgrade is possible, a two minor skew is only possible if the worker pool version is set to control plane version and then the control plane is updated gradually two minor version.
  - if the version is removed from the worker pool, only one minor version difference is allowed to the control plane.

Automatic updates of kubernetes versions also apply to worker pool kubernetes versions.
