package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	applyconfigurationscorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	"regexp"
	"strings"
)

type priorities map[int][]*regexp.Regexp

func (prio priorities) createPriorityIndexIfAbsent(priorityValue int) (created bool) {
	if _, ok := prio[priorityValue]; !ok {
		prio[priorityValue] = make([]*regexp.Regexp, 0)
		created = true
	}

	return created
}

func (prio priorities) AddNpIfNotExists(np []string, priorityValue int) {
	i := 0
	prio.createPriorityIndexIfAbsent(priorityValue)
	j := len(prio[priorityValue])

	prio[priorityValue] = append(prio[priorityValue], make([]*regexp.Regexp, len(np))...)

	for _, target := range np {
		reg, err := regexp.Compile(".*" + regexp.QuoteMeta(target) + ".*")
		if err == nil {
			found := false
			for k := range prio[priorityValue] {
				if prio[priorityValue][k] == nil {
					break
				}
				if prio[priorityValue][k].String() == reg.String() {
					found = true
					break
				}
			}
			if !found {
				prio[priorityValue][i+j] = reg
				i++
			}
		}
	}

	k := j + i
	prio[priorityValue] = prio[priorityValue][0:k]
}

func (prio priorities) RemoveNpIfExists(np []string, priorityValue int) {
	j := 0
	if !prio.createPriorityIndexIfAbsent(priorityValue) {
		toKeepPrs := make([]*regexp.Regexp, len(prio[priorityValue]))

		for _, existingRegexp := range prio[priorityValue] {
			exists := false
			for _, src := range np {
				reg, err := regexp.Compile(".*" + regexp.QuoteMeta(src) + ".*")
				if err == nil {
					if existingRegexp.String() == reg.String() {
						exists = true
						break
					}
				}
			}
			if !exists {
				toKeepPrs[j] = existingRegexp
				j++
			}

		}
		prio[priorityValue] = toKeepPrs[0:j]
	}
}

func (prio priorities) SerializePriorities() serializablePriorities {
	var ser serializablePriorities = make(map[int][]string, len(prio))
	for priorityLevel, patterns := range prio {
		ser[priorityLevel] = make([]string, len(patterns))
		for i, pattern := range patterns {
			ser[priorityLevel][i] = pattern.String()
		}
	}

	return ser
}

type serializablePriorities map[int][]string
type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value bool   `json:"value"`
}

const (
	// PriorityConfigMapName defines a name of the ConfigMap used to store priority expander configuration
	PriorityConfigMapName = "cluster-autoscaler-priority-expander"
	// ConfigMapKey defines the key used in the ConfigMap to configure priorities
	ConfigMapKey = "priorities"
)

var (
	kubeconfig    = flag.String("kubeconfig", "", "Kubeconfig file absolute path")
	prioritizeNp  = flag.String("prioritize-np", "test-p2,test-p3", "Nodepools list that will be prioritized, comma separated for several ones")
	cordonNp      = flag.String("cordon-np", "test-p1", "Nodepools to cordon (mark unschedulable in spec), comma separated for several ones")
	nodePoolLabel = flag.String("nodepool-label", "cloud.google.com/gke-nodepool", "Nodepool selector label key")
	priorityValue = flag.Int("priority-value", 100, "priority-value defines the priority level to set for np regexps that will be prioritized to receive workloads")
	priorityCmNs  = flag.String("priority-cm-ns", "kube-system", "The namespace where is living cluster-autoscaler-priority-expander configmap")
	undo          = flag.Bool("undo", false, "Undo")
	config        *rest.Config
)

func npInputCleaning(input *string) []string {
	var cleanNpList []string
	nps := strings.Split(*input, ",")

	for _, np := range nps {
		if np != "" {
			cleanNpList = append(cleanNpList, np)
		}
	}

	return cleanNpList
}

func parsePrioritiesYAMLString(prioritiesYAML string) (priorities, error) {
	if prioritiesYAML == "" {
		return nil, fmt.Errorf("priority configuration in %s configmap is empty; please provide a valid configuration",
			PriorityConfigMapName)
	}
	var config map[int][]string
	if err := yaml.Unmarshal([]byte(prioritiesYAML), &config); err != nil {
		return nil, fmt.Errorf("Can't parse YAML with priorities in the configmap: %v", err)
	}

	newPriorities := make(map[int][]*regexp.Regexp)
	for prio, reList := range config {
		for _, re := range reList {
			regexp, err := regexp.Compile(re)
			if err != nil {
				return nil, fmt.Errorf("Can't compile regexp rule for priority %d and rule %s: %v", prio, re, err)
			}
			newPriorities[prio] = append(newPriorities[prio], regexp)
		}
	}

	klog.V(4).Info("Successfully loaded priorities configuration from configmap")

	return newPriorities, nil
}

func GetClientset() (cs *kubernetes.Clientset, retErr error) {

	var err error

	if *kubeconfig == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			retErr = errors.New(err.Error())
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			retErr = errors.New(fmt.Sprintf("Error with config file `%s`, %s", *kubeconfig, err))
		}
	}

	cs, err = kubernetes.NewForConfig(config)
	if err != nil {
		retErr = errors.New(fmt.Sprintf("Bad config file `%s`", err))
	} else {

	}

	return cs, retErr
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()

	clientset, err := GetClientset()

	if err != nil {
		klog.Fatalf("Unable to get a proper configuration file to query a kube api server: %s", err)
	} else {
		klog.V(4).Info("Clientset properly retrieved")
	}

	var prs priorities

	core := clientset.CoreV1()
	klog.V(4).Infof("Trying to retrieve `%s` configmap in `%s` namespace", PriorityConfigMapName, *priorityCmNs)
	prioExpanderCm, err := core.ConfigMaps(*priorityCmNs).Get(ctx, PriorityConfigMapName, metav1.GetOptions{})

	if err == nil {
		klog.V(4).Infof("Configmap retrieved! Now trying to parse `%s` key", ConfigMapKey)
		prs, err = parsePrioritiesYAMLString(prioExpanderCm.Data[ConfigMapKey])
		if err != nil {
			klog.Infof("Unable to parse priorities string value at key `%s` in existing `%s` configmap, we will build a new one", ConfigMapKey, PriorityConfigMapName)
			prs = make(map[int][]*regexp.Regexp)
		}
	} else {
		klog.Infof("Unable to get `%s` configmap, we will create a new one", PriorityConfigMapName)
		prs = make(map[int][]*regexp.Regexp)
	}

	klog.Infof("Initial `%s` configmap `%s` key state: %s", PriorityConfigMapName, ConfigMapKey, prs)

	klog.V(4).Infof("Before cleaning `prioritize-np` input value: %s", *prioritizeNp)
	klog.V(4).Infof("Before cleaning `cordon-np` input value: %s", *cordonNp)
	klog.V(4).Info("Cleaning `prioritize-np` and `cordon-np` inputs")
	prioritizeAr := npInputCleaning(prioritizeNp)
	cordonAr := npInputCleaning(cordonNp)
	klog.V(4).Infof("Cleaned `prioritize-np` value: %s", prioritizeAr)
	klog.V(4).Infof("Cleaned `cordon-np` value: %s", cordonAr)

	if (*undo) {
		klog.Infof("About to remove %s from priorities cm", prioritizeAr)
		prs.RemoveNpIfExists(prioritizeAr, *priorityValue)
	} else {
		klog.V(4).Infof("Removing cordon nodepools %s from %s", cordonAr, prs)
		prs.RemoveNpIfExists(cordonAr, *priorityValue)
		klog.V(4).Infof("Adding prioritize nodepools %s to %s", prioritizeAr, prs)
		prs.AddNpIfNotExists(prioritizeAr, *priorityValue)
	}

	klog.V(4).Infof("Serializing new priorities value: %s", prs)
	strPrs := prs.SerializePriorities()
	newPrsStr, err := yaml.Marshal(&strPrs)
	if err == nil {
		klog.Infof("New serialized computed priorities value:\n%s", newPrsStr)
	}

	cmApplyConf := applyconfigurationscorev1.ConfigMap(PriorityConfigMapName, *priorityCmNs)
	cmApplyConf.Data = make(map[string]string, 1)
	cmApplyConf.Data[ConfigMapKey] = string(newPrsStr)

	klog.Infof("Will apply new computed configmap: %s", cmApplyConf)
	_, err = core.ConfigMaps(*priorityCmNs).Apply(ctx, cmApplyConf, metav1.ApplyOptions{FieldManager: "k8s-nodepool-cordon", Force: true})

	if err != nil {
		klog.Fatalf("Unable apply cm: %s", err)
	}

	unschedValue := true
	if *undo {
		klog.Infof("About to mark schedulable all nodes having label `%s` in %s", *nodePoolLabel, cordonAr)
		unschedValue = false
	} else {
		klog.Infof("About to mark unschedulable all nodes having label `%s` in %s", *nodePoolLabel, cordonAr)
	}

	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/spec/unschedulable",
		Value: unschedValue,
	}}

	klog.V(4).Infof("The patch submitted to apiserver for concerned nodes will look like: %s", payload)

	payloadBytes, err := json.Marshal(payload)

	if err != nil {
		klog.Fatalf("Unable to json marshal path: %s", err)
	}

	for _, srcNp := range cordonAr {
		klog.V(4).Infof("About to list the nodes under %s nodepool", srcNp)
		nodesLs, err := core.Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: *nodePoolLabel + "=" + srcNp,
		})
		if err != nil {
			klog.Errorf("Skipping %s, we received an error trying to retrieve matching nodes: %s", srcNp, err)
			continue
		}
		if len(nodesLs.Items) == 0 {
			klog.Infof("No nodes matching %s nodepool", srcNp)
		}
		for _, nd := range nodesLs.Items {
			klog.Infof("Patching node %s under nodepool %s", nd, srcNp)
			_, err := core.Nodes().Patch(ctx, nd.ObjectMeta.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
			if err != nil {
				klog.Errorf("Error patching node %s under nodepool %s: %s", nd, srcNp, err)
			}
		}
	}
	klog.Info("Task achieved!")
}
