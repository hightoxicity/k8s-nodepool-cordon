# k8s-nodepool-cordon
A tool to mark nodes under some nodepools unschedulable and allow to prioritize other nodepools near cluster autoscaler

# Usage

```
./k8s-nodepool-cordon -kubeconfig /path/to/.kube/config -cordon-np "mynodepool-1,mynodepool-2" -prioritize-np "mybackupnodepool-1,mybackupnodepool-2"
```

# Verbose mode

```
./k8s-nodepool-cordon -kubeconfig /path/to/.kube/config -cordon-np "mynodepool-1,mynodepool-2" -prioritize-np "mybackupnodepool-1,mybackupnodepool-2" -v=4
```
