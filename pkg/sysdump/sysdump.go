// Package sysdump collects and dumps information for troubleshooting Dev2
package sysdump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/mholt/archiver/v3"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const outputDir = "knoxctl_out/sysdump/"

// Options options for sysdump
type Options struct {
	Filename string
}

// Collect Function
func Collect(c *k8s.Client, o Options) error {
	var errs errgroup.Group

	d, err := os.MkdirTemp("", "accuknox-sysdump-")
	if err != nil {
		return err
	}

	// k8s Server Version
	errs.Go(func() error {
		v, err := c.K8sClientset.Discovery().ServerVersion()
		if err != nil {
			return err
		}
		if err := writeToFile(path.Join(d, "version.txt"), v.String()); err != nil {
			return err
		}
		return nil
	})

	errs.Go(func() error {
		pods, err := c.K8sClientset.CoreV1().Pods(common.AccuknoxAgents).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("accuknox-agents pod not found. \n")
			return nil
		}

		for _, p := range pods.Items {
			pod, err := c.K8sClientset.CoreV1().Pods(p.Namespace).Get(context.Background(), p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			var containerNames []string
			for _, container := range pod.Spec.Containers {
				containerNames = append(containerNames, container.Name)
			}
			fmt.Printf("getting logs from %s\n", p.Name)
			for _, container := range containerNames {
				logOptions := &corev1.PodLogOptions{
					Container: container,
				}
				v := c.K8sClientset.CoreV1().Pods(p.Namespace).GetLogs(p.Name, logOptions)
				s, err := v.Stream(context.Background())
				if err != nil {
					fmt.Printf("failed getting logs from pod=%s err=%s\n", p.Name, err)
					continue
				}
				defer func() {
					if err := s.Close(); err != nil {
						fmt.Printf("Error closing io stream %s\n", err)
					}
				}()
				var logs bytes.Buffer
				if _, err = io.Copy(&logs, s); err != nil {
					return err
				}
				if err := writeToFile(path.Join(d, "accuknox-agents-pod-"+p.Name+"-container-"+container+"-log.txt"), logs.String()); err != nil {
					return err
				}

			}

			if err := writeYaml(path.Join(d, "accuknox-agents-pod-"+p.Name+".yaml"), pod); err != nil {
				return err
			}

			e, err := c.K8sClientset.CoreV1().Events(p.Namespace).Search(scheme.Scheme, pod)
			if err != nil {
				return err
			}
			if err := writeYaml(path.Join(d, "accuknox-agents-pod-events-"+p.Name+".yaml"), e); err != nil {
				return err
			}
		}
		return nil
	})

	dumpError := errs.Wait()

	emptyDump, err := IsDirEmpty(d)
	if err != nil {
		return err
	}

	if emptyDump {
		return dumpError
	}

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	sysdumpFile := ""
	if o.Filename == "" {
		formattedTime := strings.ReplaceAll(time.Now().Format(time.UnixDate), ":", "_")
		formattedTime = strings.ReplaceAll(formattedTime, " ", "_")
		sysdumpFile = outputDir + formattedTime + ".zip"
	} else {
		sysdumpFile = outputDir + strings.ReplaceAll(o.Filename, " ", "_")
	}

	if err := archiver.Archive([]string{d}, sysdumpFile); err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	if err := os.RemoveAll(d); err != nil {
		return err
	}

	fmt.Printf("Sysdump at %s\n", sysdumpFile)

	if dumpError != nil {
		return dumpError
	}

	return nil
}

func writeToFile(p, v string) error {
	return os.WriteFile(p, []byte(v), 0600)
}

func writeYaml(p string, o runtime.Object) error {
	var j printers.YAMLPrinter
	w, err := printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(&j, nil)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	if err := w.PrintObj(o, &b); err != nil {
		return err
	}
	return writeToFile(p, b.String())
}

// IsDirEmpty Function
func IsDirEmpty(name string) (bool, error) {
	files, err := os.ReadDir(name)

	if err != nil {
		return false, err
	}

	if len(files) != 0 {
		return false, nil
	}

	return true, nil
}
