// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package summary shows observability data from discovery engine
package summary

import (
	"context"
	"errors"
	"fmt"
	"github.com/accuknox/accuknox-cli-v2/pkg"
	"github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/clarketm/json"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultReqType : default option for request type
var DefaultReqType = "process,file,network"

// Options Structure
type Options struct {
	GRPC         string
	Labels       []string
	Namespace    []string
	Clusters     []string
	Operation    string
	Workloads    []Workload
	Source       []string
	Destination  []string
	Output       string
	RevDNSLookup bool
}

type Workload struct {
	Type string
	Name string

	Namespace string
	Cluster   string
}

// GetSummary on pods
func GetSummary(c *k8s.Client, o Options) (*summary.SummaryResponse, error) {

	gRPC, err := pkg.ConnectGrpc(c, o.GRPC)

	data := &summary.SummaryRequest{
		Labels:     o.Labels,
		Namespaces: o.Namespace,
		Clusters:   o.Clusters,
		Operation:  o.Operation,
		//WorkloadTypes: []*summary.Workload{},
		Source:      o.Source,
		Destination: o.Destination,
	}

	// create a client
	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- kubectl get po -n accuknox-agents")
	}
	defer conn.Close()

	client := summary.NewSummaryClient(conn)

	//sumResp, err := o.getSummary(client, data)

	sumResp, err := o.getSummaryPerWorkload(client, data)
	if err != nil {
		return nil, err
	}

	return sumResp, nil

}

// Summary - printing the summary output
func Summary(c *k8s.Client, o Options) error {

	_, err := GetSummary(c, o)
	if err != nil {
		log.Error().Msgf("error while getting summary, error: %s", err.Error())
		return err
	}

	return nil
}

func (o *Options) getSummary(client summary.SummaryClient, sumReq *summary.SummaryRequest) (*summary.SummaryResponse, error) {
	sumResp, err := client.GetSummaryEvent(context.Background(), sumReq)

	if err != nil {
		return nil, err
	}

	return sumResp, nil
}

func (o *Options) getSummaryPerWorkload(client summary.SummaryClient, sumReq *summary.SummaryRequest) (*summary.SummaryResponse, error) {

	// TODO: apply options for clusters,namespaces, labels and workloads here itself.
	workloadReq := &summary.WorkloadRequest{}

	workloads, err := client.GetWorkloads(context.Background(), workloadReq)
	if err != nil {
		log.Error().Msgf("error while fetching summary, error: %s", err.Error())
		return nil, err
	}

	for _, w := range workloads.Workloads {
		sumReq.Namespaces = []string{w.Namespace}
		sumReq.Clusters = []string{w.Cluster}
		sumResp, err := client.GetSummaryEvent(context.Background(), sumReq)
		if err != nil {
			log.Error().Msgf("error while fetching summary for workload: %s in namespace : %s, error: %s", w.Name, w.Namespace, err.Error())
			continue
		}
		err = printOutput(o.Output, sumResp, o.Operation, o.RevDNSLookup)
		if err != nil {
			log.Error().Msgf("error while displaying summary, error: %s", err.Error())
			//return nil, err
		}
	}
	return nil, nil
}

func printOutput(outputType string, summaryResp *summary.SummaryResponse, operationType string, revDns bool) error {
	if outputType == "json" {
		summaryByte, err := json.MarshalIndent(summaryResp, "", "  ")
		if err != nil {
			log.Error().Msgf("error while marshalling summary, error: %s", err.Error())
			return err
		}
		fmt.Println(string(summaryByte))
	} else {
		for clusterName, cluster := range summaryResp.GetClusters() {
			for nsName, namespace := range cluster.GetNamespaces() {

				for depName, dep := range namespace.Deployments {
					pkg.DisplayOutput(dep.Events, revDns, operationType, clusterName, nsName, "Deployment", depName)
				}
				for dsName, ds := range namespace.DaemonSets {
					pkg.DisplayOutput(ds.Events, revDns, operationType, clusterName, nsName, "Deployment", dsName)
				}
				for rsName, rs := range namespace.ReplicaSets {
					pkg.DisplayOutput(rs.Events, revDns, operationType, clusterName, nsName, "Deployment", rsName)
				}
				for stsName, sts := range namespace.StatefulSets {
					pkg.DisplayOutput(sts.Events, revDns, operationType, clusterName, nsName, "Deployment", stsName)
				}
				for cjName, cj := range namespace.CronJobs {
					pkg.DisplayOutput(cj.Events, revDns, operationType, clusterName, nsName, "Deployment", cjName)
				}
				for jobName, job := range namespace.Jobs {
					pkg.DisplayOutput(job.Events, revDns, operationType, clusterName, nsName, "Deployment", jobName)
				}
			}
		}
	}
	return nil
}
