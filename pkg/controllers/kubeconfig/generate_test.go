package kubeconfig

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/openshift/cluster-capi-operator/pkg/controllers"
)

var _ = Describe("Generate kubeconfig", func() {
	var options *kubeconfigOptions

	BeforeEach(func() {
		options = &kubeconfigOptions{
			token:            []byte("dGVzdA=="),
			caCert:           []byte("dGVzdA=="),
			apiServerEnpoint: "https://example.com",
			clusterName:      "test",
		}
	})

	It("should generate kubeconfig", func() {
		kubeconfig, err := generateKubeconfig(*options)
		Expect(err).NotTo(HaveOccurred())
		Expect(kubeconfig).NotTo(BeNil())

		Expect(kubeconfig.Clusters).To(HaveKey(options.clusterName))
		Expect(kubeconfig.Clusters[options.clusterName].Server).To(Equal(options.apiServerEnpoint))
		Expect(kubeconfig.Clusters[options.clusterName].CertificateAuthorityData).To(Equal(options.caCert))

		Expect(kubeconfig.Contexts).To(HaveKey(options.clusterName))
		Expect(kubeconfig.Contexts[options.clusterName].Cluster).To(Equal(options.clusterName))
		Expect(kubeconfig.Contexts[options.clusterName].AuthInfo).To(Equal("cluster-capi-operator"))
		Expect(kubeconfig.Contexts[options.clusterName].Namespace).To(Equal(controllers.DefaultManagedNamespace))

		Expect(kubeconfig.AuthInfos).To(HaveKey("cluster-capi-operator"))
		Expect(kubeconfig.AuthInfos["cluster-capi-operator"].Token).To(Equal("test"))
	})

	It("should fail with empty token", func() {
		options.token = nil
		kubeconfig, err := generateKubeconfig(*options)
		Expect(err).To((HaveOccurred()))
		Expect(kubeconfig).To(BeNil())
	})

	It("should fail with empty ca cert", func() {
		options.caCert = nil
		kubeconfig, err := generateKubeconfig(*options)
		Expect(err).To((HaveOccurred()))
		Expect(kubeconfig).To(BeNil())
	})

	It("should fail with empty api server endpoint", func() {
		options.apiServerEnpoint = ""
		kubeconfig, err := generateKubeconfig(*options)
		Expect(err).To((HaveOccurred()))
		Expect(kubeconfig).To(BeNil())
	})

	It("should fail with empty cluster name", func() {
		options.clusterName = ""
		kubeconfig, err := generateKubeconfig(*options)
		Expect(err).To((HaveOccurred()))
		Expect(kubeconfig).To(BeNil())
	})
})
