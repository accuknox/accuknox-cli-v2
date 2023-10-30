package recommend

import (
	"context"
	"errors"
	"strings"

	// Karmor
	"github.com/kubearmor/kubearmor-client/k8s"
	"sigs.k8s.io/yaml"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	dev2policy "github.com/accuknox/dev2/api/grpc/v1/policy"
	hardening "github.com/accuknox/dev2/hardening/pkg/types"
	"github.com/fatih/color"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var connection *grpc.ClientConn

func initClientConnection(c *k8s.Client) error {
	if connection != nil {
		return nil
	}
	var err error
	connection, err = getClientConnection(c)
	if err != nil {
		return err
	}
	log.Info("Connected to discovery engine")
	return nil
}

func closeConnectionToDiscoveryEngine() {
	if connection != nil {
		err := connection.Close()
		if err != nil {
			log.Println("Error while closing connection")
		} else {
			log.Info("Connection to discovery engine closed successfully!")
		}
	}
}

func getClientConnection(c *k8s.Client) (*grpc.ClientConn, error) {
	gRPC, err := common.ConnectGrpc(c, "")
	if err != nil {
		log.WithError(err).Error("failed to connect to gRPC")
		return nil, err
	}

	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- Create a portforward to discovery engine service using\n\t\033[1mkubectl port-forward -n explorer service/knoxautopolicy --address 0.0.0.0 --address :: 9089:9089\033[0m\n[0m")
	}
	return conn, nil
}

//
//func recommendAdmissionControllerPolicies(img ImageInfo) error {
//	client := worker.NewWorkerClient(connection)
//	labels := libs.LabelMapToString(img.Labels)
//	resp, err := client.Convert(context.Background(), &worker.WorkerRequest{
//		Labels:     labels,
//		Namespace:  img.Namespace,
//		Policytype: "AdmissionControllerPolicy",
//	})
//	if err != nil {
//		color.Red(err.Error())
//		return err
//	}
//	if resp.AdmissionControllerPolicy != nil {
//		for _, policy := range resp.AdmissionControllerPolicy {
//			var kyvernoPolicy kyvernov1.Policy
//			err := json.Unmarshal(policy.Data, &kyvernoPolicy)
//			if err != nil {
//				return err
//			}
//			if namespaceMatches(kyvernoPolicy.Namespace) && matchAdmissionControllerPolicyTags(&kyvernoPolicy) {
//				img.writeAdmissionControllerPolicy(kyvernoPolicy)
//			}
//		}
//	}
//	return nil
//}

func recommendHardeningPolicy(img ImageInfo) error {
	client := dev2policy.NewGetPolicyClient(connection)
	resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
		Type:      "hardening",
		Kind:      "KubeArmorPolicy",
		TenantId:  0,
		ClusterId: 0,
	})
	if err != nil {
		color.Red(err.Error())
		return err
	}

	if resp.Policies != nil {
		for _, policy := range resp.Policies {
			policyStr := string(policy.Yaml)

			var hardeningPolicy hardening.KubeArmorPolicy
			err := yaml.Unmarshal([]byte(policyStr), &hardeningPolicy)
			if err != nil {
				log.Error(err)
				return err
			}

			if namespaceMatches(hardeningPolicy.Metadata.Namespace) && matchHardeningTags(&hardeningPolicy) {
				img.writeHardeningPolicy(hardeningPolicy)
			}
		}
	}

	return nil
}

func matchAdmissionControllerPolicyTags(policy *kyvernov1.Policy) bool {
	policyTags := strings.Split(policy.Annotations["recommended-policies.kubearmor.io/tags"], ",")
	if len(options.Tags) <= 0 {
		return true
	}
	for _, t := range options.Tags {
		if slices.Contains(policyTags, t) {
			return true
		}
	}
	return false
}

func matchHardeningTags(policy *hardening.KubeArmorPolicy) bool {
	policyTags := policy.Spec.Tags
	if len(options.Labels) <= 0 {
		return true
	}
	for _, t := range options.Tags {
		if slices.Contains(policyTags, t) {
			return true
		}
	}

	return false
}

func namespaceMatches(policyNamespace string) bool {
	return options.Namespace == "" || options.Namespace == policyNamespace
}
