package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[BasicClusterPropagation] basic cluster propagation testing", func() {
	ginkgo.Context("CustomResourceDefinition propagation testing", func() {
		crdGroup := fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
		randStr := rand.String(RandomStrLength)
		crdSpecNames := apiextensionsv1.CustomResourceDefinitionNames{
			Kind:     fmt.Sprintf("Foo%s", randStr),
			ListKind: fmt.Sprintf("Foo%sList", randStr),
			Plural:   fmt.Sprintf("foo%ss", randStr),
			Singular: fmt.Sprintf("foo%s", randStr),
		}
		crd := helper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.NamespaceScoped)
		crdPolicy := helper.NewClusterPropagationPolicy(crd.Name, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: crd.APIVersion,
				Kind:       crd.Kind,
				Name:       crd.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
		})
		crdGVR := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating crdPolicy(%s)", crdPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), crdPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing crdPolicy(%s)", crdPolicy.Name), func() {
				err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Delete(context.TODO(), crdPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("crd propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating crd(%s)", crd.Name), func() {
				unstructObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Create(context.TODO(), &unstructured.Unstructured{Object: unstructObj}, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("get crd(%s)", crd.Name), func() {
				_, err := dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if crd present on member clusters", func() {
				for _, cluster := range clusters {
					clusterDynamicClient := getClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for crd(%s) present on cluster(%s)", crd.Name, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing crd(%s)", crd.Name), func() {
				err := dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Delete(context.TODO(), crd.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if crd disappeared from member clusters", func() {
				for _, cluster := range clusters {
					clusterDynamicClient := getClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for crd(%s) disappeared on cluster(%s)", crd.Name, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}
						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})
})
