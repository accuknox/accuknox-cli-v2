package discoveryengine

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	pb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/license"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var matchLabels = map[string]string{"app": "discovery-engine"}
var port int64 = 9089
var cursorcount int

func InstallLicense(client *k8s.Client, key string, user string) error {
	gRPC := ""
	targetSvc := "discovery-engine"

	if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
		gRPC = val
	} else {
		pf, err := utils.InitiatePortForward(client, port, port, matchLabels, targetSvc)
		if err != nil {
			return err
		}
		gRPC = "localhost:" + strconv.FormatInt(pf.LocalPort, 10)
	}

	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	licenseClient := pb.NewLicenseClient(conn)

	req := &pb.LicenseInstallRequest{
		Key:    key,
		UserId: user,
	}
	_, err = licenseClient.InstallLicense(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("🥳  License installed successfully for discovery engine.\n")

	return nil
}

func CheckPods(client *k8s.Client) int {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("\r😋   Checking if DiscoveryEngine pods are running ...")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := client.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "discovery-engine", FieldSelector: "status.phase!=Running"})
		podno := len(pods.Items)
		clearLine(90)
		fmt.Printf("\rDiscovery Engine pods left to run : %d ... %s", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == 4 {
			cursorcount = 0
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️  Check Incomplete due to Time-Out!                     \n")
			break
		}
		if podno == 0 {
			fmt.Printf("\r🥳  Done Checking , ALL Services are running!             \n")
			fmt.Printf("⌚️  Execution Time : %s \n", time.Since(stime))
			break
		}
	}
	return 0
}

func clearLine(size int) int {
	for i := 0; i < size; i++ {
		fmt.Printf(" ")
	}
	fmt.Printf("\r")
	return 0
}
