// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	operator_catalog "code.cloudfoundry.org/cf-operator/testing"
	testing_utils "code.cloudfoundry.org/quarks-utils/testing"
	"context"
	eirinix "github.com/SUSE/eirinix"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/watch"
	"os"
	"strconv"
)

// NewCatalog returns a Catalog, our helper for test cases
func NewCatalog() Catalog {
	port, err := freeport.GetFreePort()
	if err != nil {
		panic(err) // Cannot allocate free ports! everything will fail!
	}
	return Catalog{Catalog: &operator_catalog.Catalog{}, ServicePort: int32(port), KindHost: "172.17.0.1"}
}

// NewContext returns a non-nil empty context, for usage when it is unclear
// which context to use.  Mostly used in tests.
func NewContext() context.Context {
	return testing_utils.NewContext()
}

// Catalog provides several instances for test, based on the cf-operator's catalog
type Catalog struct {
	*operator_catalog.Catalog
	ServicePort int32
	KindHost    string
}

// SimpleExtension it's returning a fake dummy Eirini extension
func (c *Catalog) SimpleExtension() eirinix.Extension {

	return &testExtension{
		parentExtension{Name: "test"}}
}

// SimpleManager returns a dummy Extensions manager
func (c *Catalog) SimpleManager() eirinix.Manager {
	return eirinix.NewManager(
		eirinix.ManagerOptions{
			Namespace: "namespace",
			Host:      "127.0.0.1",
			Port:      90,
		})
}

// IntegrationManager returns an Extensions manager which is used by integration tests
func (c *Catalog) IntegrationManager() eirinix.Manager {
	return eirinix.NewManager(
		eirinix.ManagerOptions{
			Namespace:        "default",
			Host:             c.KindHost,
			Port:             c.ServicePort,
			KubeConfig:       os.Getenv("KUBECONFIG"),
			ServiceName:      "eirinix",
			WebhookNamespace: "default",
		})
}

// IntegrationManagerFiltered returns an Extensions manager which is used by integration tests which filters or not eirini apps
func (c *Catalog) IntegrationManagerFiltered(b bool, n string) eirinix.Manager {
	return eirinix.NewManager(
		eirinix.ManagerOptions{
			Namespace:        n,
			Host:             c.KindHost,
			Port:             c.ServicePort,
			KubeConfig:       os.Getenv("KUBECONFIG"),
			ServiceName:      "eirinix",
			WebhookNamespace: n,
			FilterEiriniApps: &b,
		})
}

// IntegrationManagerNoRegister returns an Extensions manager which is used by integration tests, which doesn't register extensions again
func (c *Catalog) IntegrationManagerNoRegister() eirinix.Manager {
	RegisterWebhooks := false
	return eirinix.NewManager(
		eirinix.ManagerOptions{
			Namespace:        "default",
			Host:             c.KindHost,
			Port:             c.ServicePort,
			KubeConfig:       os.Getenv("KUBECONFIG"),
			ServiceName:      "eirinix",
			WebhookNamespace: "default",
			RegisterWebHook:  &RegisterWebhooks,
		})
}

// ServiceYaml returns the yaml of the endpoint + service used to reach eiriniX returned in IntegrationManager
func (c *Catalog) ServiceYaml() []byte {
	return []byte(`
apiVersion: v1
kind: Service
metadata:
  name: eirinix
spec:
  ports:
  - protocol: TCP
    port: 443
    targetPort: ` + strconv.Itoa(int(c.ServicePort)) + `
---
apiVersion: v1
kind: Endpoints
metadata:
  name: eirinix
subsets:
  - addresses:
      - ip: ` + c.KindHost + `
    ports:
      - port: ` + strconv.Itoa(int(c.ServicePort)) + `
`)
}

// EiriniAppYaml returns a fake Eirini app yaml
func (c *Catalog) EiriniAppYaml() []byte {
	return []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: eirini-fake-app
  labels:
    ` + eirinix.LabelSourceType + `: APP
spec:
  containers:
  - image: busybox:1.28.4
    command:
      - sleep
      - "3600"
    name: eirini-fake-app
    env:
    - name: FAKE_APP
      value: "fake content"
  restartPolicy: Always
`)
}

// EiriniStagingAppYaml returns a fake Eirini staging app yaml
func (c *Catalog) EiriniStagingAppYaml() []byte {
	return []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: 6ad9f634-b32e-4890-b1ba-55202d95bc3a-xdcp6
spec:
  containers:
  - image: busybox:1.28.4
    command:
      - sleep
      - "3600"
    name: 6ad9f634-b32e-4890-b1ba-55202d95bc3a-xdcp6
  restartPolicy: Always
`)
}

// RegisterEiriniXService register the service generated in ServiceYaml()
func (c *Catalog) RegisterEiriniXService() error {

	err := KubeApply(c.ServiceYaml())
	if err != nil {
		return err
	}

	return nil
}

type EiriniApp struct {
	Name, Namespace string
	Pod             *Pod
}

// StartEiriniApp starts EiriniAppYaml with kubernetes
func (c *EiriniApp) IsRunning() (bool, error) {
	p, err := KubePodStatus(c.Name, c.Namespace)
	if err != nil {
		return false, err
	}
	return p.IsRunning(), nil
}

func (c *EiriniApp) Delete() error {
	out, err := Kubectl([]string{}, "delete", "pod", "-n", c.Namespace, c.Name)
	if err != nil {
		return errors.Wrap(err, "Failed: "+string(out))
	}
	return nil
}

func (c *EiriniApp) Sync() error {
	p, err := KubePodStatus(c.Name, c.Namespace)
	if err != nil {
		return err
	}
	c.Pod = p
	return nil
}

// StartEiriniApp starts EiriniAppYaml with kubernetes
func (c *Catalog) StartEiriniApp() (*EiriniApp, error) {

	err := KubeApply(c.EiriniAppYaml())
	if err != nil {
		return nil, err
	}

	return &EiriniApp{Name: "eirini-fake-app", Namespace: "default"}, nil
}

// StartEiriniApp starts EiriniAppYaml with kubernetes
func (c *Catalog) StartEiriniStagingApp() (*EiriniApp, error) {

	err := KubeApply(c.EiriniStagingAppYaml())
	if err != nil {
		return nil, err
	}

	return &EiriniApp{Name: "6ad9f634-b32e-4890-b1ba-55202d95bc3a-xdcp6", Namespace: "default"}, nil
}

// StartEiriniApp starts EiriniAppYaml with kubernetes
func (c *Catalog) StartEiriniAppInNamespace(n string) (*EiriniApp, error) {

	err := KubeApplyNamespace(c.EiriniAppYaml(), n)
	if err != nil {
		return nil, err
	}

	return &EiriniApp{Name: "eirini-fake-app", Namespace: n}, nil
}

// StartEiriniApp starts EiriniAppYaml with kubernetes
func (c *Catalog) StartEiriniStagingAppInNamespace(n string) (*EiriniApp, error) {

	err := KubeApplyNamespace(c.EiriniStagingAppYaml(), n)
	if err != nil {
		return nil, err
	}

	return &EiriniApp{Name: "6ad9f634-b32e-4890-b1ba-55202d95bc3a-xdcp6", Namespace: n}, nil
}

// SimpleManagerService returns a dummy Extensions manager configured to run as a service
func (c *Catalog) SimpleManagerService() eirinix.Manager {
	return eirinix.NewManager(
		eirinix.ManagerOptions{
			Namespace:        "eirini",
			Host:             "0.0.0.0",
			Port:             0,
			ServiceName:      "extension",
			WebhookNamespace: "cf",
		})
}

type SimpleWatch struct {
	Handled []watch.Event
}

func (sw *SimpleWatch) Handle(m eirinix.Manager, e watch.Event) {
	sw.Handled = append(sw.Handled, e)
}

// SimpleWatcher returns a dummy watcher
func (c *Catalog) SimpleWatcherWithChannel(channel chan watch.Event) eirinix.Watcher {
	return &SimpleWatcherWithChannel{Received: channel}
}

type SimpleWatcherWithChannel struct {
	Received chan watch.Event
}

func (sw *SimpleWatcherWithChannel) Handle(m eirinix.Manager, e watch.Event) {
	sw.Received <- e
}

// SimpleWatcher returns a dummy watcher
func (c *Catalog) SimpleWatcher() eirinix.Watcher {
	return &SimpleWatch{}
}
