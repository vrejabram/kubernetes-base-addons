package test

import (
	"fmt"
	"github.com/blang/semver"
	"github.com/mesosphere/kubeaddons/hack/temp"
	"github.com/mesosphere/kubeaddons/pkg/repositories/local"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"testing"

	"github.com/mesosphere/kubeaddons/pkg/api/v1beta1"
	"github.com/mesosphere/kubeaddons/pkg/test"
	"github.com/mesosphere/kubeaddons/pkg/test/cluster/kind"
	"github.com/mesosphere/kubeaddons/pkg/test/cluster/konvoy"
)

type TestGroup struct {
	ClusterInterface  string   `yaml:"clusterInterface,omitempty"`
	KubernetesVersion string   `yaml:"kubernetesVersion,omitempty"`
	KonvoyProvisioner string   `yaml:"konvoyProvisioner,omitempty"`
	Addons            []string `yaml:"addons,omitempty"`
}

const (
	defaultKonvoyPath = "/Users/jared/git/konvoy/out-darwin/konvoy"
)

var addonTestingGroups = make(map[string]TestGroup)

func init() {
	b, err := ioutil.ReadFile("groups.yaml")
	if err != nil {
		panic(err)
	}

	if err := yaml.Unmarshal(b, addonTestingGroups); err != nil {
		panic(err)
	}
}

func TestValidateUnhandledAddons(t *testing.T) {
	unhandled, err := findUnhandled()
	if err != nil {
		t.Fatal(err)
	}

	if len(unhandled) != 0 {
		names := make([]string, len(unhandled))
		for _, addon := range unhandled {
			names = append(names, addon.GetName())
		}
		t.Fatal(fmt.Errorf("the following addons are not handled as part of a testing group: %+v", names))
	}
}

func TestGeneralGroup(t *testing.T) {
	if err := testgroup(t, "general"); err != nil {
		t.Fatal(err)
	}
}

func TestElasticSearchGroup(t *testing.T) {
	if err := testgroup(t, "elasticsearch"); err != nil {
		t.Fatal(err)
	}
}

func TestPrometheusGroup(t *testing.T) {
	if err := testgroup(t, "prometheus"); err != nil {
		t.Fatal(err)
	}
}

func TestKommanderGroup(t *testing.T) {
	if err := testgroup(t, "kommander"); err != nil {
		t.Fatal(err)
	}
}

func TestKonvoyGeneralGroup(t *testing.T) {
	if err := testgroup(t, "konvoyGeneral"); err != nil {
		t.Fatal(err)
	}
}

func TestIstioGroup(t *testing.T) {
	if err := testgroup(t, "istio"); err != nil {
		t.Fatal(err)
	}
}

func TestKonvoySecurityGroup(t *testing.T) {
	if err := testgroup(t, "konvoySecurity"); err != nil {
		t.Fatal(err)
	}
}

func TestKonvoyCertManagerGroup(t *testing.T) {
	if err := testgroup(t, "konvoyCertManager"); err != nil {
		t.Fatal(err)
	}
}

// -----------------------------------------------------------------------------
// Private Functions
// -----------------------------------------------------------------------------

func testgroup(t *testing.T, groupname string) error {
	t.Logf("testing group %s", groupname)

	testGroup, ok := addonTestingGroups[groupname]
	if !ok {
		return fmt.Errorf("%s group does not exist in groups.yaml", groupname)
	}
	// Get correct interface here
	cluster, err := getClusterFromGroup(testGroup)
	if err != nil {
		return err
	}
	defer cluster.Cleanup()

	if err := temp.DeployController(cluster, testGroup.ClusterInterface); err != nil {
		return err
	}

	addons, err := addons(addonTestingGroups[groupname].Addons...)
	if err != nil {
		return err
	}

	ph, err := test.NewBasicTestHarness(t, cluster, addons...)
	if err != nil {
		return err
	}
	defer ph.Cleanup()

	ph.Validate()
	ph.Deploy()

	return nil
}

func addons(names ...string) ([]v1beta1.AddonInterface, error) {
	var testAddons []v1beta1.AddonInterface
	repo, err  := local.NewRepository("base", "../addons")
	if err != nil {
		return testAddons, err
	}
	addons, err := repo.ListAddons()
	if err != nil {
		return testAddons, err
	}

	for _, addon := range addons {
		for _, name := range names {
			overrides(addon[0])
			if addon[0].GetName() == name {
				testAddons = append(testAddons, addon[0])
			}
		}
	}

	if len(testAddons) != len(names) {
		return testAddons, fmt.Errorf("got %d addons, expected %d", len(testAddons), len(names))
	}

	return testAddons, nil
}

func findUnhandled() ([]v1beta1.AddonInterface, error) {
	var unhandled []v1beta1.AddonInterface

	repo, err := local.NewRepository("base", "../addons")
	if err != nil {
		return unhandled, err
	}

	addons, err := repo.ListAddons()
	if err != nil {
		return unhandled, err
	}

	for _, revisions := range addons {
		addon := revisions[0]
		found := false
		for _, v := range addonTestingGroups {
			for _, name := range v.Addons {
				if name == addon.GetName() {
					found = true
				}
			}
		}
		if !found {
			unhandled = append(unhandled, addon)
		}
	}

	return unhandled, nil
}

// -----------------------------------------------------------------------------
// Private - CI Values Overrides
// -----------------------------------------------------------------------------

// TODO: a temporary place to put configuration overrides for addons
// See: https://jira.mesosphere.com/browse/DCOS-62137
func overrides(addon v1beta1.AddonInterface) {
	if v, ok := addonOverrides[addon.GetName()]; ok {
		addon.GetAddonSpec().ChartReference.Values = &v
	}
}

var addonOverrides = map[string]string{
	"metallb": `
---
configInline:
  address-pools:
  - name: default
    protocol: layer2
    addresses:
    - "172.17.1.200-172.17.1.250"
`,
	"istio": `---
      kiali:
       enabled: true
       contextPath: /ops/portal/kiali
       ingress:
         enabled: true
         kubernetes.io/ingress.class: traefik
         hosts:
           - ""
       dashboard:
         auth:
           strategy: anonymous
       prometheusAddr: http://prometheus-kubeaddons-prom-prometheus.kubeaddons:9090

      tracing:
        enabled: true
        contextPath: /ops/portal/jaeger
        ingress:
          enabled: true
          kubernetes.io/ingress.class: traefik
          hosts:
            - ""

      grafana:
        enabled: true

      prometheus:
        serviceName: prometheus-kubeaddons-prom-prometheus.kubeaddons

      istiocoredns:
        enabled: true

      security:
       selfSigned: true
       caCert: /etc/cacerts/tls.crt
       caKey: /etc/cacerts/tls.key
       rootCert: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
       certChain: /etc/cacerts/tls.crt
       enableNamespacesByDefault: false

      global:
       podDNSSearchNamespaces:
       - global
       - "{{ valueOrDefault .DeploymentMeta.Namespace \"default\" }}.global"

       mtls:
        enabled: true

       multiCluster:
        enabled: true

       controlPlaneSecurityEnabled: true
`,
}

func getClusterFromGroup(group TestGroup) (test.Cluster, error) {

	if group.ClusterInterface == "kind" {
		return kind.NewCluster(semver.MustParse(group.KubernetesVersion))
	} else if group.ClusterInterface == "konvoy" {

		return konvoy.NewKonvoyCluster(getKonvoyPath(), group.KonvoyProvisioner)
	}
	return nil, fmt.Errorf("'%s' is not a supported clusterInterface", group.ClusterInterface)
}

func getKonvoyPath() string {
	if konvoyPath := os.Getenv("KONVOY_PATH"); konvoyPath != "" {
		return konvoyPath
	}
	return defaultKonvoyPath
}
