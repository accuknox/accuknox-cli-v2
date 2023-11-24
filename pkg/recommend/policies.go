package recommend

import (
	"context"
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/schollz/progressbar/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v2"

	dev2policy "github.com/accuknox/dev2/api/grpc/v1/policy"
	policyType "github.com/accuknox/dev2/hardening/pkg/types"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getKaPolicy(c *k8s.Client, o *Options) ([]string, error) {
	fmt.Println("Generating recommended hardening policies...")
	policyBucket := NewPolicyBucket()

	var bar *progressbar.ProgressBar
	var data []string

	gRPC, err := common.ConnectGrpc(c, o.Grpc)
	if err != nil {
		log.WithError(err).Error("failed to init gRPC connection")
		return nil, err
	}
	connection, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.WithError(err).Error("failed to connect to discovery engine")
		return nil, err
	}
	defer connection.Close()

	client := dev2policy.NewGetPolicyClient(connection)

	fetchPolicies := func(nsFilter string) error {
		var policyRequest = &dev2policy.PolicyRequest{
			Type: PolicyType,
			Kind: KindKubeArmorPolicy,
		}

		if nsFilter != "" {
			policyRequest.Namespace = nsFilter
		}

		resp, err := client.GetPolicy(context.Background(), policyRequest)
		if err != nil {
			return err
		}
		if resp != nil {
			if bar == nil {
				bar = initializeProgressBar(len(resp.Policies)) // Initialize the progress bar
			}

			for _, policy := range resp.Policies {
				policyString := string(policy.Yaml)
				var kaPolicy policyType.KubeArmorPolicy
				err := yaml.Unmarshal([]byte(policyString), &kaPolicy)
				if err != nil {
					log.WithError(err).Error("failed to unmarshal policy")
				}

				data = append(data, policyString)
				policyBucket.AddPolicy(policy.Namespace, &kaPolicy)

				bar.Add(1)
			}
		}
		return nil
	}

	getAllPolicies := func() error {
		resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
			Type: PolicyType,
			Kind: KindKubeArmorPolicy,
		})
		if err != nil {
			return err
		}
		if resp != nil {
			if bar == nil {
				bar = initializeProgressBar(len(resp.Policies)) // Initialize the progress bar
			}

			for _, policy := range resp.Policies {
				policyString := string(policy.Yaml)
				var kaPolicy policyType.KubeArmorPolicy
				err := yaml.Unmarshal([]byte(policyString), &kaPolicy)
				if err != nil {
					return err
				}

				data = append(data, policyString)
				policyBucket.AddPolicy(policy.Namespace, &kaPolicy)

				bar.Add(1)
			}

		}

		return nil
	}

	if o.noFilter() {
		err := getAllPolicies()
		if err != nil {
			log.WithError(err).Error("failed to fetch all policies")
			return nil, err
		}
	} else {
		var allNamespaces []string
		var err error
		if len(o.NamespaceRegex) > 0 {
			allNamespaces, err = getAllNamespaces(c)
			if err != nil {
				log.WithError(err).Error("failed to get namespaces")
				return nil, err
			}
		} else {
			allNamespaces = o.Namespace
		}

		for _, ns := range allNamespaces {
			if len(o.NamespaceRegex) > 0 {
				for _, regex := range o.NamespaceRegex {
					if regex.MatchString(ns) {
						err := fetchPolicies(ns)
						if err != nil {
							log.WithError(err).Error("failed to fetch policy for namespace regex " + regex.String())
							continue
						}
					}
				}
			} else {
				err := fetchPolicies(ns)
				if err != nil {
					log.WithError(err).Error("failed to fetch policy for namespace " + ns)
					continue
				}
			}
		}
	}

	val, err := policyBucket.RetrievePolicies(c, o)
	if err != nil {
		return nil, err
	}

	if len(val) == 0 {
		fmt.Println("No hardening policies found. Please check discovery engine logs.")
		return nil, nil
	}

	if bar != nil {
		bar.Finish()
	}

	if o.Dump {
		dump(policyBucket)
		return nil, nil
	}

	StartTUI(policyBucket)
	return data, nil
}

func getAllNamespaces(client *k8s.Client) ([]string, error) {
	namespaceList, err := client.K8sClientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.WithError(err).Error("failed to list namespaces")
		return nil, err
	}

	var namespaces []string
	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	return namespaces, nil
}

func initializeProgressBar(totalCount int) *progressbar.ProgressBar {
	bar := progressbar.NewOptions(
		totalCount,
		progressbar.OptionSetDescription("Processing policies..."),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSpinnerType(9),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowIts(),
	)
	return bar
}
