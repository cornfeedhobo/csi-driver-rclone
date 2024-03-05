package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cornfeedhobo/csi-driver-rclone/internal/csirclone"
	"github.com/cornfeedhobo/csi-driver-rclone/internal/kclient"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var (
	defaultMountOpt = map[string]string{
		"AllowOther":   "true",
		"AsyncRead":    "true",
		"AttrTimeout":  time.Second.String(),
		"MaxReadAhead": strconv.Itoa(128 * 1024),
	}

	defaultVfsOpt = map[string]string{}

	driverOpt = &csirclone.DriverOptions{}

	secretName string

	cmd = &cobra.Command{
		Use: csirclone.DefaultDriverName,
		Run: run,
	}
)

func init() {
	klogFS := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(klogFS)
	cmd.Flags().AddGoFlagSet(klogFS)

	cmd.Flags().StringVar(&driverOpt.NodeId, "node-id", "", "node id")

	cmd.Flags().StringVar(&driverOpt.DriverName, "driver-name", csirclone.DefaultDriverName, "name of the driver")

	cmd.Flags().StringVar(&driverOpt.Endpoint, "driver-endpoint", csirclone.DefaultDriverEndpoint, "CSI endpoint")

	cmd.Flags().StringVar(&secretName, "secret-name", "", "name of the secret containing config for rclone.")

	cmd.Flags().StringVar(&driverOpt.Address, "rcd-address", "http://localhost:5572/", "the address to use when contacting rcd.")

	cmd.Flags().StringVar(&driverOpt.Username, "rcd-username", "", "the username to use when contacting rcd. required if secretname is not set.")

	cmd.Flags().StringVar(&driverOpt.Password, "rcd-password", "", "the password to use when contacting rcd. required if secretname is not set.")

	cmd.Flags().StringVar(&driverOpt.Remote, "remote", "", "rclone remote to use. required if secretname is not set.")

	cmd.Flags().StringVar(&driverOpt.MountType, "mounttype", csirclone.DefaultMountType, "rclone mount type.")

	cmd.Flags().StringToStringVar(&driverOpt.MountOpt, "mountopt", defaultMountOpt, "rclone mount options.")

	cmd.Flags().StringToStringVar(&driverOpt.VfsOpt, "vfsopt", defaultVfsOpt, "rclone vfs options.")
}

func run(cmd *cobra.Command, args []string) {

	if driverOpt.NodeId == "" {
		klog.Warning("node-id is empty")
	}

	if secretName != "" {

		klog.Infof("Pulling config from secret '%s'", secretName)

		client, err := kclient.NewClient()
		if err != nil {
			klog.Fatalf("error creating k8s client instance: %s", err)
		}

		secret, err := client.GetSecret(secretName)
		if err != nil {
			klog.Fatalf("error pulling k8s secret '%s': %s", secretName, err)
		}

		klog.Info("Parsing config secret data")

		// FIXME
		panic(secret)
	}

	if err := driverOpt.Validate(); err != nil {
		klog.Fatalf("error validating driver options: %s", err)
	}

	driver := csirclone.NewDriver(driverOpt)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		driver.Stop()
	}()

	driver.Start()
	driver.Wait()
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
