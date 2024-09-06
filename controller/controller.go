package controller

import (
	"context"
	"fmt"
	controllerv1 "github.com/RejwankabirHamim/crd/pkg/apis/books.com/v1"
	myclient "github.com/RejwankabirHamim/crd/pkg/generated/clientset/versioned"
	informer "github.com/RejwankabirHamim/crd/pkg/generated/informers/externalversions/books.com/v1"
	lister "github.com/RejwankabirHamim/crd/pkg/generated/listers/books.com/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsInformer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appsListers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"log"
	"time"
)

type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset myclient.Interface

	DeploymentsLister appsListers.DeploymentLister
	DeploymentsSynced cache.InformerSynced
	BookStoreLister   lister.BookStoreLister
	BookStoreSynced   cache.InformerSynced
	// workQueue is a rate limited work queue. This is used to queue work to be processed instead of
	// performing it as soon as a change happens. This means we can ensure we only process a fixed
	// amount of resources at a time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers
	workQueue workqueue.RateLimitingInterface
}

// NewController returns a sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset myclient.Interface,
	deploymentInformer appsInformer.DeploymentInformer,
	bookstoreInformer informer.BookStoreInformer) *Controller {
	cntrl := &Controller{
		kubeclientset:     kubeclientset,
		sampleclientset:   sampleclientset,
		DeploymentsLister: deploymentInformer.Lister(),
		DeploymentsSynced: deploymentInformer.Informer().HasSynced,
		BookStoreLister:   bookstoreInformer.Lister(),
		BookStoreSynced:   bookstoreInformer.Informer().HasSynced,
		workQueue:         workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
	log.Println("Setting up event handlers")

	// Setting up an event handler for the changes of BookStore type resources
	bookstoreInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: cntrl.enqueueBookStore,
		UpdateFunc: func(oldobj, newobj interface{}) {
			cntrl.enqueueBookStore(newobj)
		},
		DeleteFunc: func(obj interface{}) {
			cntrl.enqueueBookStore(obj)
		},
	})
	return cntrl
}

// this function is used to take kubernetes resources and
// converting it into a unique string and then adding it into the queue
func (c *Controller) enqueueBookStore(obj interface{}) {
	log.Println("Enqueuing book store")

	// key is a string that uniquely identifies obj
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workQueue.AddRateLimited(key)
}

func (c *Controller) Run(workerss int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workQueue.ShutDown()

	log.Println("Starting controller")
	log.Println("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(stopCh, c.DeploymentsSynced, c.BookStoreSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	log.Println("Starting workers")

	for i := 0; i < workerss; i++ {
		// call c.runWorker every second until stopCh is closed
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Println("Worker started successfully")
	<-stopCh
	log.Println("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.ProcessNextItem() {
		// slowing down the execution to see the changes
		time.Sleep(1 * time.Second)
	}

}

func (c *Controller) ProcessNextItem() bool {
	obj, shutdown := c.workQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// We call Done at the end of this func so the workqueue knows we have
		// finished processing this item. We also must remember to call Forget
		// if we do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off period.
		defer c.workQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workQueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the structured reference to the object to be synced.
		if err := c.syncHandler(key); err != nil {
			c.workQueue.AddRateLimited(obj)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err)
		}
		c.workQueue.Forget(obj)
		log.Printf("Successfully synced '%s'", key)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	bookStore, err := c.BookStoreLister.BookStores(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("BookStore '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}
	deploymentName := bookStore.Spec.Name
	if deploymentName == "" {
		utilruntime.HandleError(fmt.Errorf("%s : deployment name must be specified", key))
		return nil
	}

	deployment, err := c.DeploymentsLister.Deployments(bookStore.Namespace).Get(deploymentName)
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(bookStore.Namespace).Create(context.TODO(), newDeployment(bookStore), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}
	err = c.updateBookStoreStatus(bookStore)

	if err != nil {
		return err
	}

	if bookStore.Spec.Replicas != nil && *bookStore.Spec.Replicas != *deployment.Spec.Replicas {
		log.Printf("BookStore in %s has replicas: %d, deployment has replicas: %d\n", namespace, *bookStore.Spec.Replicas, *deployment.Spec.Replicas)
		deployment, err = c.kubeclientset.AppsV1().Deployments(namespace).Update(context.TODO(), newDeployment(bookStore), metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	serviceName := bookStore.Spec.Name + "-service"
	service, err := c.kubeclientset.CoreV1().Services(bookStore.Namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(bookStore.Namespace).Create(context.TODO(), newService(bookStore), metav1.CreateOptions{})
		if err != nil {
			return err
		}
		log.Printf("%s service has been created\n", serviceName)
	} else if err != nil {
		return err
	}
	_, err = c.kubeclientset.CoreV1().Services(bookStore.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) updateBookStoreStatus(bookStore *controllerv1.BookStore) error {
	bookStoreCopy := bookStore.DeepCopy()
	//bookStoreCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas

	// setting the replica count to a random a value to see
	// if the deployment specs also changes according to it
	y := int32(rand.Intn(10) + 1)
	bookStoreCopy.Spec.Replicas = &y
	fmt.Println("y: ", y)

	_, err := c.sampleclientset.BookStoreV1().BookStores(bookStore.Namespace).Update(context.TODO(), bookStoreCopy, metav1.UpdateOptions{})
	return err

}

func newDeployment(bookStore *controllerv1.BookStore) *appsv1.Deployment {
	fmt.Println("Inside newDeployment +++++++++++")

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bookStore.Spec.Name,
			Namespace: bookStore.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: bookStore.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "bookserver",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "bookserver",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "bookapi",
							Image: "hamim99/book-server:latest",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: bookStore.Spec.Container.Port,
								},
							},
						},
					},
				},
			},
		},
	}
}

func newService(bookStore *controllerv1.BookStore) *corev1.Service {
	fmt.Println("---+++--- ", bookStore.Spec.Container.Port)
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bookStore.Spec.Name + "-service",
			Namespace: bookStore.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bookStore, controllerv1.SchemeGroupVersion.WithKind("BookStore")),
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "bookserver",
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       4000,
					TargetPort: intstr.FromInt32(bookStore.Spec.Container.Port),
				},
			},
		},
	}
}
