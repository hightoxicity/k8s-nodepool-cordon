# k8s-nodepool-cordon
A tool to mark nodes under some nodepools unschedulable and allow to prioritize other nodepools near cluster autoscaler

# Usage

## Binary usage

```
./k8s-nodepool-cordon -kubeconfig /path/to/.kube/config -cordon-np "mynodepool-1,mynodepool-2" -prioritize-np "mybackupnodepool-1,mybackupnodepool-2"
```

## Docker usage

```
docker run --mount type=bind,source="${HOME}"/.kube/config,target=/root/.kube/config djnos/k8s-nodepool-cordon:v1.0.2 -cordon-np "mynodepool-1,mynodepool-2" -prioritize-np "mybackupnodepool-1,mybackupnodepool-2"
```

# Verbose mode

```
./k8s-nodepool-cordon -kubeconfig /path/to/.kube/config -cordon-np "mynodepool-1,mynodepool-2" -prioritize-np "mybackupnodepool-1,mybackupnodepool-2" -v=4
```
