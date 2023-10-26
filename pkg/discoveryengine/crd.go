package discoveryengine

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCRD() *apiextv1.CustomResourceDefinition {
	crd := &apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "discoveredpolicies.security.kubearmor.com",
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: "security.kubearmor.com",
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:     "DiscoveredPolicy",
				ListKind: "DiscoveredPolicyList",
				Plural:   "discoveredpolicies",
				Singular: "discoveredpolicy",
			},
			Scope: apiextv1.NamespaceScoped,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Description: "DiscoveredPolicy is the Schema for the discoveredpolicies API",
							Type:        "object",
							Properties: map[string]apiextv1.JSONSchemaProps{
								"apiVersion": {
									Type: "string",
								},
								"kind": {
									Type: "string",
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Description: "DiscoveredPolicySpec defines the desired state of DiscoveredPolicy",
									Type:        "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"policy": {
											XPreserveUnknownFields: boolPtr(true),
										},
										"status": {
											Default: apiextInactivePtr(),
											Type:    "string",
											Enum: []apiextv1.JSON{
												{Raw: []byte("\"Inactive\"")},
												{Raw: []byte("\"inactive\"")},
												{Raw: []byte("\"Active\"")},
												{Raw: []byte("\"active\"")},
												{Raw: []byte("\"PendingUpdates\"")},
											},
										},
									},
									Required: []string{"status"},
								},
								"status": {
									Description: "DiscoveredPolicyStatus defines the observed state of DiscoveredPolicy",
									Type:        "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"kind": {
											Type: "string",
										},
										"lastUpdatedTime": {
											Format: "date-time",
											Type:   "string",
										},
										"message": {
											Type: "string",
										},
										"phase": {
											Type: "string",
											Enum: []apiextv1.JSON{
												{Raw: []byte("\"Validated\"")},
												{Raw: []byte("\"Success\"")},
												{Raw: []byte("\"Failed\"")},
												{Raw: []byte("\"Unknown\"")},
											},
										},
										"reason": {
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return crd
}
