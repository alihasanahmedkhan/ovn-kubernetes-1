package ovn

import (
	goovn "github.com/ebay/go-ovn"
	. "github.com/onsi/gomega"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	ovntest "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/testing"
	util "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"

	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	egressfirewallfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressfirewall/v1/apis/clientset/versioned/fake"
	egressipfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressip/v1/apis/clientset/versioned/fake"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
)

const (
	k8sTCPLoadBalancerIP  = "k8s_tcp_load_balancer"
	k8sUDPLoadBalancerIP  = "k8s_udp_load_balancer"
	k8sSCTPLoadBalancerIP = "k8s_sctp_load_balancer"
	fakeUUID              = "8a86f6d8-7972-4253-b0bd-ddbef66e9303"
	fakeUUIDv6            = "8a86f6d8-7972-4253-b0bd-ddbef66e9304"
)

type FakeOVN struct {
	fakeClient       *fake.Clientset
	fakeEgressClient *egressfirewallfake.Clientset
	fakeCRDClient    *apiextensionsfake.Clientset
	watcher          *factory.WatchFactory
	controller       *Controller
	stopChan         chan struct{}
	fakeExec         *ovntest.FakeExec
	asf              *fakeAddressSetFactory
	ovnNBClient      goovn.Client
	ovnSBClient      goovn.Client
}

func NewFakeOVN(fexec *ovntest.FakeExec) *FakeOVN {
	err := util.SetExec(fexec)
	Expect(err).NotTo(HaveOccurred())
	return &FakeOVN{
		fakeExec: fexec,
		asf:      newFakeAddressSetFactory(),
	}
}

func (o *FakeOVN) start(ctx *cli.Context, objects ...runtime.Object) {
	_, err := config.InitConfig(ctx, o.fakeExec, nil)
	Expect(err).NotTo(HaveOccurred())

	o.fakeCRDClient = apiextensionsfake.NewSimpleClientset()
	o.fakeClient = fake.NewSimpleClientset(objects...)
	o.init()
}

func (o *FakeOVN) restart() {
	o.shutdown()
	o.init()
}

func (o *FakeOVN) shutdown() {
	close(o.stopChan)
	if o.fakeEgressClient != nil {
		o.watcher.ShutdownEgressFirewallWatchFactory()
	}
	o.watcher.Shutdown()
	err := o.controller.ovnNBClient.Close()
	Expect(err).NotTo(HaveOccurred())
	err = o.controller.ovnSBClient.Close()
	Expect(err).NotTo(HaveOccurred())
}

func (o *FakeOVN) init() {
	var err error

	o.stopChan = make(chan struct{})
	egressIPFakeClient := &egressipfake.Clientset{}
	o.watcher, err = factory.NewWatchFactory(o.fakeClient, egressIPFakeClient, o.fakeEgressClient, o.fakeCRDClient)
	if o.fakeEgressClient != nil {
		o.watcher.InitializeEgressFirewallWatchFactory()
	}
	Expect(err).NotTo(HaveOccurred())
	o.ovnNBClient = ovntest.NewMockOVNClient(goovn.DBNB)
	o.ovnSBClient = ovntest.NewMockOVNClient(goovn.DBSB)
	o.controller = NewOvnController(o.fakeClient, egressIPFakeClient, o.fakeEgressClient, o.watcher,
		o.stopChan, o.asf, o.ovnNBClient,
		o.ovnSBClient)
	o.controller.multicastSupport = true

}

func mockAddNBDBError(table, name, field string, err error, ovnNBClient goovn.Client) {
	mockClient, ok := ovnNBClient.(*ovntest.MockOVNClient)
	if !ok {
		panic("type assertion failed for mock NB client")
	}
	mockClient.AddToErrorCache(table, name, field, err)
}

func mockAddSBDBError(table, name, field string, err error, ovnSBClient goovn.Client) {
	mockClient, ok := ovnSBClient.(*ovntest.MockOVNClient)
	if !ok {
		panic("type assertion failed for mock SB client")
	}
	mockClient.AddToErrorCache(table, name, field, err)
}

func mockDelNBDBError(table, name, field string, ovnNBClient goovn.Client) {
	mockClient, ok := ovnNBClient.(*ovntest.MockOVNClient)
	if !ok {
		panic("type assertion failed for mock NB client")
	}
	mockClient.RemoveFromErrorCache(table, name, field)
}

func mockDelSBDBError(table, name, field string, ovnSBClient goovn.Client) {
	mockClient, ok := ovnSBClient.(*ovntest.MockOVNClient)
	if !ok {
		panic("type assertion failed for mock SB client")
	}
	mockClient.RemoveFromErrorCache(table, name, field)
}
