// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package aibom

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/bedrock/types"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
)

// BedrockOptions holds configuration for AIBOM generation from AWS Bedrock.
type BedrockOptions struct {
	// Region is the AWS region to query, e.g. "us-east-1".
	Region string

	// UseDefaultCredentials instructs the generator to use the standard AWS
	// credential chain (env vars → ~/.aws/credentials → IAM role).
	// When false, AccessKeyID + SecretAccessKey must be provided.
	UseDefaultCredentials bool

	// Explicit credentials — only used when UseDefaultCredentials is false.
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string // optional

	// ModelID filters to a single foundation model when set.
	// When empty, all available foundation models are inventoried.
	ModelID string

	// Metadata overrides
	Name         string
	Version      string
	Manufacturer string

	// Output
	OutputTo string
	Format   string
}

// GenerateFromBedrock calls the AWS Bedrock API to list/describe foundation
// models and returns a CycloneDX BOM inventorying them as ML model components.
func GenerateFromBedrock(opts *BedrockOptions) (*cdx.BOM, error) {
	if opts.Region == "" {
		return nil, fmt.Errorf("--region is required for AWS Bedrock")
	}

	ctx := context.Background()

	// Build AWS config.
	var awsCfg aws.Config
	var err error
	if opts.UseDefaultCredentials || opts.AccessKeyID == "" {
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(opts.Region))
	} else {
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(opts.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				opts.AccessKeyID, opts.SecretAccessKey, opts.SessionToken,
			)),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	client := bedrock.NewFromConfig(awsCfg)

	// Fetch models.
	var models []types.FoundationModelSummary
	if opts.ModelID != "" {
		// Single model — use GetFoundationModel.
		out, err := client.GetFoundationModel(ctx, &bedrock.GetFoundationModelInput{
			ModelIdentifier: aws.String(opts.ModelID),
		})
		if err != nil {
			return nil, fmt.Errorf("fetching Bedrock model %q: %w", opts.ModelID, err)
		}
		d := out.ModelDetails
		models = []types.FoundationModelSummary{{
			ModelId:                    d.ModelId,
			ModelName:                  d.ModelName,
			ProviderName:               d.ProviderName,
			InputModalities:            d.InputModalities,
			OutputModalities:           d.OutputModalities,
			CustomizationsSupported:    d.CustomizationsSupported,
			InferenceTypesSupported:    d.InferenceTypesSupported,
			ResponseStreamingSupported: d.ResponseStreamingSupported,
			ModelLifecycle:             d.ModelLifecycle,
		}}
	} else {
		// All models — use ListFoundationModels.
		out, err := client.ListFoundationModels(ctx, &bedrock.ListFoundationModelsInput{})
		if err != nil {
			return nil, fmt.Errorf("listing Bedrock foundation models: %w", err)
		}
		models = out.ModelSummaries
	}

	return buildBedrockBOM(models, opts), nil
}

// buildBedrockBOM constructs a CycloneDX BOM from a slice of Bedrock model summaries.
func buildBedrockBOM(models []types.FoundationModelSummary, opts *BedrockOptions) *cdx.BOM {
	bom := cdx.NewBOM()
	bom.SerialNumber = "urn:uuid:" + uuid.New().String()
	bom.JSONSchema = jsonSchema17

	// Use first model (or the only model) as the BOM subject when there's one.
	subjectName := "AWS Bedrock Foundation Models"
	subjectRef := "aws-bedrock-models"
	if len(models) == 1 && models[0].ModelId != nil {
		subjectName = aws.ToString(models[0].ModelName)
		if subjectName == "" {
			subjectName = aws.ToString(models[0].ModelId)
		}
		subjectRef = aws.ToString(models[0].ModelId)
	}

	bom.Metadata = &cdx.Metadata{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Lifecycles: &[]cdx.Lifecycle{{Phase: cdx.LifecyclePhaseBuild}},
		Tools: &cdx.ToolsChoice{
			Components: &[]cdx.Component{toolComponent},
		},
		Component: &cdx.Component{
			BOMRef: subjectRef,
			Type:   cdx.ComponentTypeMachineLearningModel,
			Name:   subjectName,
		},
	}

	comps := make([]cdx.Component, 0, len(models))
	for _, m := range models {
		comps = append(comps, bedrockModelComponent(m, opts))
	}

	bom.Components = &comps
	enforceRequiredFields(bom)
	return bom
}

// bedrockModelComponent converts a single Bedrock FoundationModelSummary into a
// CycloneDX ML model component.
func bedrockModelComponent(m types.FoundationModelSummary, opts *BedrockOptions) cdx.Component {
	modelID := aws.ToString(m.ModelId)
	name := aws.ToString(m.ModelName)
	if name == "" {
		name = modelID
	}
	if opts.Name != "" {
		name = opts.Name
	}
	provider := aws.ToString(m.ProviderName)
	if opts.Manufacturer != "" {
		provider = opts.Manufacturer
	}
	version := opts.Version // optional override; Bedrock models have no concept of git SHA

	purl := fmt.Sprintf("pkg:generic/aws-bedrock/%s", modelID)
	if version != "" {
		purl += "@" + version
	}
	bomRef := purl

	comp := cdx.Component{
		BOMRef:      bomRef,
		Type:        cdx.ComponentTypeMachineLearningModel,
		Name:        name,
		Group:       provider,
		Version:     version,
		PackageURL:  purl,
		Description: bedrockDescription(m),
	}

	if provider != "" {
		comp.Supplier = &cdx.OrganizationalEntity{Name: provider}
	}

	// Lifecycle hint as a tag.
	var tags []string
	if m.ModelLifecycle != nil {
		tags = append(tags, "lifecycle:"+strings.ToLower(string(m.ModelLifecycle.Status)))
	}
	for _, mod := range m.InputModalities {
		tags = append(tags, "input:"+strings.ToLower(string(mod)))
	}
	for _, mod := range m.OutputModalities {
		tags = append(tags, "output:"+strings.ToLower(string(mod)))
	}
	if len(tags) > 0 {
		comp.Tags = &tags
	}

	// Properties for static model attributes.
	var modelProps []cdx.Property
	addProp := func(name, value string) {
		if value != "" {
			modelProps = append(modelProps, cdx.Property{Name: name, Value: value})
		}
	}
	if m.ResponseStreamingSupported != nil {
		addProp("responseStreamingSupported", fmt.Sprintf("%v", *m.ResponseStreamingSupported))
	}
	if len(m.InferenceTypesSupported) > 0 {
		types_ := make([]string, len(m.InferenceTypesSupported))
		for i, t := range m.InferenceTypesSupported {
			types_[i] = string(t)
		}
		addProp("inferenceTypesSupported", strings.Join(types_, ", "))
	}
	if len(m.CustomizationsSupported) > 0 {
		customs := make([]string, len(m.CustomizationsSupported))
		for i, c := range m.CustomizationsSupported {
			customs[i] = string(c)
		}
		addProp("customizationsSupported", strings.Join(customs, ", "))
	}
	if m.ModelLifecycle != nil {
		addProp("lifecycleStatus", strings.ToLower(string(m.ModelLifecycle.Status)))
	}
	if len(modelProps) > 0 {
		comp.Properties = &modelProps
	}

	// ModelCard with input/output modalities as the task description.
	comp.ModelCard = bedrockModelCard(m)

	// External reference to AWS Bedrock docs page.
	refs := []cdx.ExternalReference{
		{
			Type:    cdx.ERTypeWebsite,
			URL:     "https://docs.aws.amazon.com/bedrock/latest/userguide/models-supported.html",
			Comment: "AWS Bedrock supported models",
		},
	}
	comp.ExternalReferences = &refs

	// Licenses — Bedrock models are proprietary; express this clearly.
	comp.Licenses = &cdx.Licenses{
		{License: &cdx.License{ID: "LicenseRef-AWS-Bedrock-EULA"}},
	}

	return comp
}

// bedrockDescription builds a human-readable description for a Bedrock model.
func bedrockDescription(m types.FoundationModelSummary) string {
	var parts []string
	if p := aws.ToString(m.ProviderName); p != "" {
		parts = append(parts, "Provider: "+p)
	}
	if len(m.InputModalities) > 0 {
		mods := make([]string, len(m.InputModalities))
		for i, mod := range m.InputModalities {
			mods[i] = string(mod)
		}
		parts = append(parts, "Input: "+strings.Join(mods, ", "))
	}
	if len(m.OutputModalities) > 0 {
		mods := make([]string, len(m.OutputModalities))
		for i, mod := range m.OutputModalities {
			mods[i] = string(mod)
		}
		parts = append(parts, "Output: "+strings.Join(mods, ", "))
	}
	if m.ResponseStreamingSupported != nil && *m.ResponseStreamingSupported {
		parts = append(parts, "Streaming: yes")
	}
	if len(m.InferenceTypesSupported) > 0 {
		types_ := make([]string, len(m.InferenceTypesSupported))
		for i, t := range m.InferenceTypesSupported {
			types_[i] = string(t)
		}
		parts = append(parts, "Inference: "+strings.Join(types_, ", "))
	}
	if len(parts) == 0 {
		return "AWS Bedrock foundation model"
	}
	return strings.Join(parts, ". ")
}

// bedrockModelCard builds the CycloneDX MLModelCard for a Bedrock model.
func bedrockModelCard(m types.FoundationModelSummary) *cdx.MLModelCard {
	params := &cdx.MLModelParameters{}
	paramsEmpty := true

	// Map input modalities to a task hint and typed I/O parameters.
	if len(m.InputModalities) > 0 || len(m.OutputModalities) > 0 {
		var ins, outs []string
		for _, mod := range m.InputModalities {
			ins = append(ins, strings.ToLower(string(mod)))
		}
		for _, mod := range m.OutputModalities {
			outs = append(outs, strings.ToLower(string(mod)))
		}
		params.Task = strings.Join(ins, "+") + " → " + strings.Join(outs, "+")
		paramsEmpty = false

		// Typed I/O parameters.
		var inputParams []cdx.MLInputOutputParameters
		for _, mod := range m.InputModalities {
			inputParams = append(inputParams, cdx.MLInputOutputParameters{Format: strings.ToLower(string(mod))})
		}
		if len(inputParams) > 0 {
			params.Inputs = &inputParams
		}

		var outputParams []cdx.MLInputOutputParameters
		for _, mod := range m.OutputModalities {
			outputParams = append(outputParams, cdx.MLInputOutputParameters{Format: strings.ToLower(string(mod))})
		}
		if len(outputParams) > 0 {
			params.Outputs = &outputParams
		}
	}

	// Customisations available.
	if len(m.CustomizationsSupported) > 0 {
		parts := make([]string, len(m.CustomizationsSupported))
		for i, c := range m.CustomizationsSupported {
			parts[i] = string(c)
		}
		params.ArchitectureFamily = strings.Join(parts, ", ")
		paramsEmpty = false
	}

	// Considerations: ethical + performance tradeoffs.
	var cons cdx.MLModelCardConsiderations
	consEmpty := true

	// EULA restriction — always present for Bedrock models.
	ethics := []cdx.MLModelCardEthicalConsideration{
		{
			Name:               "Proprietary Model",
			MitigationStrategy: "Usage is governed by the AWS Bedrock Acceptable Use Policy and model-specific EULAs; review terms before production deployment",
		},
	}

	// Lifecycle warning.
	if m.ModelLifecycle != nil && strings.ToLower(string(m.ModelLifecycle.Status)) == "legacy" {
		ethics = append(ethics, cdx.MLModelCardEthicalConsideration{
			Name:               "Legacy Model",
			MitigationStrategy: "This model is in legacy status; consider migrating to a current generation model for continued support",
		})
	}
	cons.EthicalConsiderations = &ethics
	consEmpty = false

	// Performance tradeoffs from customisation support.
	if len(m.CustomizationsSupported) > 0 {
		tradeoffs := []string{
			"Fine-tuning and customisation are available but may affect base model behaviour and safety properties",
		}
		cons.PerformanceTradeoffs = &tradeoffs
	}

	mc := &cdx.MLModelCard{}
	if !paramsEmpty {
		mc.ModelParameters = params
	}
	if !consEmpty {
		mc.Considerations = &cons
	}
	if mc.ModelParameters == nil && mc.Considerations == nil {
		return nil
	}
	return mc
}
