package csirclone

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/rc"
	"golang.org/x/net/context"
	"k8s.io/klog/v2"
)

const (
	DefaultDriverName     = "rclone.csi.k8s.io"
	DefaultDriverEndpoint = "unix:///tmp/csi.sock"
	DefaultMountType      = "mount2"

	volumeOperationAlreadyExistsFmt = "an operation with the given Volume ID %s already exists"
)

type DriverOptions struct {
	DriverName string
	NodeId     string
	Endpoint   string

	Address  string
	Username string
	Password string

	Remote    string
	MountType string

	MountOpt map[string]string
	VfsOpt   map[string]string
}

func (o *DriverOptions) Validate() (err error) {

	switch {
	case o.DriverName == "":
		err = errors.New("invalid DriverOptions: DriverName required")
	case o.NodeId == "":
		err = errors.New("invalid DriverOptions: NodeId required")
	case o.Endpoint == "":
		err = errors.New("invalid DriverOptions: Endpoint required")
	case o.Remote == "":
		err = errors.New("invalid DriverOptions: Remote required")
	case o.MountType == "":
		err = errors.New("invalid DriverOptions: MountType required")
	}

	if o.Username != "" || o.Password != "" {
		switch {
		case o.Username == "":
			err = errors.New("invalid DriverOptions: Username required")
		case o.Password == "":
			err = errors.New("invalid DriverOptions: Password required")
		}
	}

	return
}

type Driver struct {
	*DriverOptions
	Version string
	WorkDir string
	Server  NonBlockingGRPCServer
	Locks   *VolumeLocks
}

func NewDriver(opts *DriverOptions) (d *Driver) {
	klog.Infof(
		"Driver: %v Version: %v",
		opts.DriverName,
		driverVersion,
	)

	d = &Driver{
		DriverOptions: opts,
		Version:       driverVersion,
		WorkDir:       path.Join(os.TempDir(), opts.DriverName),
		Server:        NewNonBlockingGRPCServer(),
		Locks:         NewVolumeLocks(),
	}

	return d
}

func marshalOpt(opt map[string]string) (string, error) {

	if opt == nil {
		return "", nil
	}

	out := map[string]interface{}{}

	for k, v := range opt {
		switch v {
		case "true":
			out[k] = true
		case "false":
			out[k] = false
		default:
			if i, err := strconv.Atoi(v); err == nil {
				out[k] = i
			} else if t, err := time.ParseDuration(v); err == nil {
				out[k] = t
			} else {
				out[k] = v
			}
		}
	}

	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (d *Driver) GetMountOpt() (string, error) {

	out, err := marshalOpt(d.MountOpt)
	if err != nil {
		return "", err
	}

	return out, nil
}

func (d *Driver) GetVfsOpt() (string, error) {

	out, err := marshalOpt(d.VfsOpt)
	if err != nil {
		return "", err
	}

	return out, nil
}

func (d *Driver) Start() {

	versionMeta, err := GetVersionYAML(d.DriverName)
	if err != nil {
		klog.Fatalf("%v", err)
	}

	klog.Info("\n" +
		"DRIVER INFORMATION:\n" +
		"-------------------\n" +
		versionMeta + "\n")

	d.Server = NewNonBlockingGRPCServer()

	d.Server.Start(
		d.Endpoint,
		NewIdentityServer(d),
		NewControlServer(d),
		NewNodeServer(d),
	)
}

func (d *Driver) Stop() {
	d.Server.Stop()
}

func (d *Driver) Wait() {
	d.Server.Wait()
}

// mostly copied from rclone/cmd/rc.doCall()
func (d *Driver) RC(ctx context.Context, path string, in rc.Params) (out rc.Params, err error) {

	url := d.Address + path
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	// Prep request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if d.Username != "" || d.Password != "" {
		req.SetBasicAuth(d.Username, d.Password)
	}

	// Do HTTP request
	client := fshttp.NewClient(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer fs.CheckClose(resp.Body, &err)

	// Read response
	body, err := io.ReadAll(resp.Body)
	bodyString := strings.TrimSpace(string(body))
	if err != nil {
		return nil, err
	}

	// Parse output
	out = make(rc.Params)
	err = json.NewDecoder(strings.NewReader(bodyString)).Decode(&out)
	if err != nil {
		return nil, err
	}

	// Check we got 200 OK
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("operation %q failed: %v", path, out["error"])
	}

	return
}

func (d *Driver) IsVolume(ctx context.Context, id string) (exist bool, err error) {

	out, err := d.RC(ctx, "operations/stat", rc.Params{
		"fs":     d.Remote,
		"remote": id + "/" + MetadataFilename,
		"opt":    `{"recurse": false}`,
	})
	if err != nil {
		err = fmt.Errorf("error calling operations/stat: %w", err)
	}
	if out["item"] != nil {
		exist = true
	}

	return
}

func (d *Driver) copyOrMoveFile(ctx context.Context, srcRemote, srcPath, destRemote, destPath string, move bool) error {

	op := "copy"
	if move {
		op = "move"
	}

	_, err := d.RC(ctx, "operations/"+op+"file", rc.Params{
		"srcFs":     srcRemote,
		"srcRemote": srcPath,
		"dstFs":     destRemote,
		"dstRemote": destPath,
	})

	if err != nil {
		switch {
		case strings.HasSuffix(err.Error(), "object not found"):
			err = ErrNotFound
		case strings.HasSuffix(err.Error(), "didn't find section in config file"):
			err = ErrRemoteNotFound
		default:
			err = fmt.Errorf("error copying file: %w", err)
		}
	}

	return err
}

func (d *Driver) CopyFile(ctx context.Context, srcRemote, srcPath, destRemote, destPath string) error {
	return d.copyOrMoveFile(ctx, srcRemote, srcPath, destRemote, destPath, false)
}

func (d *Driver) MoveFile(ctx context.Context, srcRemote, srcPath, destRemote, destPath string) error {
	return d.copyOrMoveFile(ctx, srcRemote, srcPath, destRemote, destPath, true)
}

// ReadVolume returns nil if no volume is found
func (d *Driver) ReadVolume(ctx context.Context, id string) (*Volume, error) {

	// create tmpfile to get a safe place to write
	tmpFile, err := os.CreateTemp(d.WorkDir, "")
	defer os.Remove(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("error closing temp file: %w", err)
	}

	// overwrite the file with the remote metadata file
	err = d.CopyFile(ctx,
		d.Remote, id+"/"+MetadataFilename,
		path.Dir(tmpFile.Name()), path.Base(tmpFile.Name()))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("error copying file with rclone: %w", err)
	}

	// re-open
	tmpFile, err = os.Open(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("error re-opening temp file: %w", err)
	}

	// read
	b, err := io.ReadAll(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("error closing metadata file: %w", err)
	}

	v := &Volume{}
	err = v.Unmarshal(b)

	return v, err
}

func (d *Driver) WriteVolume(ctx context.Context, v *Volume) error {

	b, err := v.Marshal(true)
	if err != nil {
		return err
	}
	b = append(b, []byte("\n")...)

	tmpFile, err := os.CreateTemp(d.WorkDir, "")
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}

	_, err = tmpFile.Write(b)
	if err != nil {
		return fmt.Errorf("error writing temp file: %w", err)
	}

	err = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("error writing temp file: %w", err)
	}

	err = d.MoveFile(ctx,
		path.Dir(tmpFile.Name()), path.Base(tmpFile.Name()),
		v.Remote, v.ID+"/"+MetadataFilename)

	return err
}

func (d *Driver) ExpandVolume(ctx context.Context, id string, capacity int64) error {

	v, err := d.ReadVolume(ctx, id)
	if err != nil {
		return err
	}

	v.Capacity = capacity

	return d.WriteVolume(ctx, v)
}

func (d *Driver) PurgeVolume(ctx context.Context, id string) error {

	_, err := d.RC(ctx, "operations/purge", rc.Params{
		"fs":     d.Remote,
		"remote": id,
	})

	return err

}

func (d *Driver) MountVolume(ctx context.Context, id, mountPoint string, parameters map[string]string) error {

	mountOpt, err := d.GetMountOpt()
	if err != nil {
		return err
	}

	vfsOpt, err := d.GetVfsOpt()
	if err != nil {
		return err
	}

	in := rc.Params{
		"fs":         d.Remote + "/" + id,
		"mountPoint": mountPoint,
		"mountType":  d.MountType,
		"mountOpt":   mountOpt,
		"vfsOpt":     vfsOpt,
	}
	for _, key := range []string{"mountType", "mountOpt", "vfsOpt"} {
		// allow overrides at the volume definition
		if value, ok := parameters[key]; ok {
			in[key] = value
		}
		// delete any keys with empty values
		if in[key] == "" {
			delete(in, key)
		}
	}

	// TODO: Add retry loop?
	lockKey := fmt.Sprintf("%s-%s", id, mountPoint)
	if !d.Locks.TryAcquire(lockKey) {
		return fmt.Errorf(volumeOperationAlreadyExistsFmt, id)
	}
	defer d.Locks.Release(lockKey)

	_, err = d.RC(ctx, "mount/mount", in)

	return err
}
