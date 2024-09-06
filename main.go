package main

import (
	"flag"
	"github.com/RejwankabirHamim/crd/controller"
	myclient "github.com/RejwankabirHamim/crd/pkg/generated/clientset/versioned"
	informers "github.com/RejwankabirHamim/crd/pkg/generated/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"path/filepath"
	"time"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	customClient, err := myclient.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	// we will use these factories to create informers to watch on specific resource

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	customInformerFactory := informers.NewSharedInformerFactory(customClient, time.Second*30)

	// new controller is being created
	c := controller.NewController(kubeClient, customClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		customInformerFactory.BookStore().V1().BookStores(),
	)

	//This channel is used to signal the controller to stop (e.g., during shutdown).

	stopCh := make(chan struct{})
	//The Start method begins the informer processes, which listen for changes to the respective resources.
	kubeInformerFactory.Start(stopCh)
	customInformerFactory.Start(stopCh)

	if err := c.Run(1, stopCh); err != nil {
		log.Println("error running controller:", err)
	}

}
