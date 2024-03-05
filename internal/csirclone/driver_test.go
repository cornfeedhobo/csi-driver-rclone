package csirclone_test

import (
	"errors"
	"os"
	"path"
	"strconv"
	"time"

	. "github.com/cornfeedhobo/csi-driver-rclone/internal/csirclone"
	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	_ "github.com/rclone/rclone/backend/all" // import all backends
	"github.com/rclone/rclone/cmd"
	_ "github.com/rclone/rclone/cmd/all"    // import all commands
	_ "github.com/rclone/rclone/lib/plugin" // import plugins
)

var _ = Describe("Driver", Ordered, func() {

	var driver *Driver

	var cleanTmpDirs = func() {
		os.RemoveAll(path.Join(os.TempDir(), "csi-rclone"))
		os.RemoveAll(path.Join(os.TempDir(), "csi-mount"))
		os.RemoveAll(path.Join(os.TempDir(), "csi-staging"))
		os.RemoveAll(path.Join(os.TempDir(), DefaultDriverName))
	}

	var runRcd = func() {
		tmpDir := path.Join(os.TempDir(), DefaultDriverName)
		err := os.Mkdir(tmpDir, os.ModePerm)
		if err != nil && !errors.Is(err, os.ErrExist) {
			panic(err) // fixme
		}
		fh, err := os.CreateTemp(tmpDir, "")
		if err != nil {
			panic(err) // fixme
		}
		if _, err := fh.Write([]byte("[unittest]\ntype = local\n")); err != nil {
			panic(err) // fixme
		}
		if err := fh.Close(); err != nil {
			panic(err) // fixme
		}
		args := os.Args
		go func() {
			os.Args = []string{
				"rclone",
				"rcd",
				"--rc-addr=0.0.0.0:5572",
				"--rc-no-auth",
				"--verbose=2",
				"--config=" + fh.Name(),
			}
			cmd.Main()
		}()
		time.Sleep(3 * time.Second)
		os.Args = args
	}

	var runDriver = func() {
		driver = NewDriver(&DriverOptions{
			NodeId:     "csiTest",
			DriverName: DefaultDriverName,
			Endpoint:   "unix:///tmp/csi.sock",
			Address:    "http://127.0.0.1:5572/",
			Remote:     "unittest:/tmp/csi-rclone",
			MountType:  "mount2",
			MountOpt: map[string]string{
				"AllowOther":   "true",
				"AsyncRead":    "true",
				"AttrTimeout":  time.Second.String(),
				"MaxReadAhead": strconv.Itoa(128 * 1024),
			},
		})
		driver.Start()
	}

	var stopDriver = func() {
		driver.Stop()
		driver.Wait()
	}

	BeforeAll(func() {
		cleanTmpDirs()
		runRcd()
		runDriver()
	})

	AfterAll(func() {
		stopDriver()
	})

	AfterEach(func() {
		// break up the log output for easier debugging
		println()
	})

	Describe("CSI sanity", func() {
		config := sanity.NewTestConfig()
		config.Address = "unix:/tmp/csi.sock"
		sanity.GinkgoTest(&config)
	})
})
