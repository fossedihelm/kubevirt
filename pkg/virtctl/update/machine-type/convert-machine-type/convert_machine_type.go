package convertmachinetype

import (
	"fmt"
	"os"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	k6tv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/controller"
)

func Run() {
	// check env variables and set them accordingly
	var (
		err           error
		targetNs      = metav1.NamespaceAll
		labelSelector = labels.Everything()
		restartNow    bool
	)

	machineTypeEnv, exists := os.LookupEnv("MACHINE_TYPE")
	if !exists {
		fmt.Println("No machine type was specified.")
		os.Exit(1)
	}

	restartEnv, exists := os.LookupEnv("RESTART_NOW")
	if exists {
		restartNow, err = strconv.ParseBool(restartEnv)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	namespaceEnv, exists := os.LookupEnv("NAMESPACE")
	if exists && namespaceEnv != "" {
		targetNs = namespaceEnv
	}

	fmt.Println("Setting label selector")
	selectorEnv, exists := os.LookupEnv("LABEL_SELECTOR")
	if exists {
		ls, err := labels.ConvertSelectorToLabelsMap(selectorEnv)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		labelSelector, err = ls.AsValidatedSelector()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// set up JobController
	virtCli, err := getVirtCli()
	if err != nil {
		os.Exit(1)
	}

	var vmListWatcher *cache.ListWatch
	var vmiListWatcher *cache.ListWatch

	vmListWatcher = controller.NewListWatchFromClient(virtCli.RestClient(), "virtualmachines", targetNs, fields.Everything(), labelSelector)
	vmiListWatcher = controller.NewListWatchFromClient(virtCli.RestClient(), "virtualmachineinstances", targetNs, fields.Everything(), labelSelector)

	vmInformer := cache.NewSharedIndexInformer(vmListWatcher, &k6tv1.VirtualMachine{}, 1*time.Hour, cache.Indexers{})
	vmiInformer := cache.NewSharedIndexInformer(vmiListWatcher, &k6tv1.VirtualMachineInstance{}, 1*time.Hour, cache.Indexers{})

	jobController, err := NewJobController(vmInformer, vmiInformer, virtCli, machineTypeEnv, restartNow)
	if err != nil {
		os.Exit(1)
	}

	go jobController.run(jobController.exitJobChan)
	<-jobController.exitJobChan
	os.Exit(0)
}

func getVirtCli() (kubecli.KubevirtClient, error) {
	clientConfig, err := kubecli.GetKubevirtClientConfig()
	if err != nil {
		return nil, err
	}

	virtCli, err := kubecli.GetKubevirtClientFromRESTConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return virtCli, err
}
