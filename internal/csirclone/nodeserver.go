package csirclone

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cornfeedhobo/csi-driver-rclone/internal/csicommon"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
)

// NodeServer is responsible for managing the mounts and status of each node.
type NodeServer struct {
	*csicommon.NodeServer

	driver  *Driver
	mounter mount.Interface

	caps []*csi.NodeServiceCapability

	// A map storing all volumes with ongoing operations so that additional operations
	// for that same volume (as defined by VolumeID) return an Aborted error
	vl *VolumeLocks
}

// NewNodeServer returns a working node server.
func NewNodeServer(d *Driver) *NodeServer {

	ns := &NodeServer{
		NodeServer: &csicommon.NodeServer{},
		driver:     d,
		mounter:    mount.New(""),
		vl:         NewVolumeLocks(),
	}

	ns.SetCapabilities([]csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
		csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
	})

	return ns
}

func (ns *NodeServer) SetCapabilities(caps []csi.NodeServiceCapability_RPC_Type) {
	var nsc []*csi.NodeServiceCapability
	for _, cap := range caps {
		klog.Infof("Enabling Node Capability %s", cap)
		nsc = append(nsc, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}
	ns.caps = nsc
}

// NodeExpandVolume node expand volume
func (ns *NodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {

	id := req.GetVolumeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	exist, err := ns.driver.IsVolume(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !exist {
		return nil, status.Error(codes.NotFound, "specified volume does not exist")
	}

	capacity := req.GetCapacityRange()
	if capacity == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capacity missing in request")
	}

	if err := ns.driver.ExpandVolume(ctx, id, capacity.RequiredBytes); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, status.Error(codes.NotFound, "specified volume does not exist")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: capacity.RequiredBytes,
	}, nil
}

// NodeGetCapabilities return the capabilities of the Node plugin
func (ns *NodeServer) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.caps,
	}, nil
}

// NodeGetInfo return info of the node on which this plugin is running
func (ns *NodeServer) NodeGetInfo(_ context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: ns.driver.NodeId,
	}, nil
}

// NodeGetVolumeStats get volume stats
func (ns *NodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {

	id := req.GetVolumeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	path := req.GetVolumePath()
	if path == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume path missing in request")
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "error getting stat of %s: %s", path, err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage:           []*csi.VolumeUsage{{Used: info.Size()}},
		VolumeCondition: &csi.VolumeCondition{Message: "healthy"},
	}, nil
}

// NodePublishVolume mount the volume from staging to target path
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {

	id := req.GetVolumeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}

	notMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			klog.V(2).Infof("NodePublishVolume: creating targetPath: %s", targetPath)
			if err := os.MkdirAll(targetPath, 0770); err != nil {
				return nil, status.Errorf(codes.Internal, "error making targetPath: %s", err)
			}
			notMnt = true
			err = nil
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if !notMnt {
		klog.V(2).Infof("NodePublishVolume: targetPath already exists: %s", targetPath)
		if _, err := os.ReadDir(targetPath); err == nil {
			klog.V(2).Infof("NodePublishVolume: targetPath already mounted: %s", targetPath)
			return &csi.NodePublishVolumeResponse{}, nil
		}

		klog.V(2).Infof("NodePublishVolume: targetPath %s is assumed to be a mount, but is not responding to ReadDir, attempting to unmount", targetPath)

		if err := ns.mounter.Unmount(targetPath); err != nil {
			klog.V(2).Infof("NodePublishVolume: Unmount directory %s failed with %v", targetPath, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	klog.V(2).Infof("NodePublishVolume: mounting %s", targetPath)
	err = ns.driver.MountVolume(ctx, id, targetPath, req.GetVolumeContext())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmount the volume from the target path
func (ns *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {

	id := req.GetVolumeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	// TODO: Add retry loop?
	lockKey := fmt.Sprintf("%s-%s", id, targetPath)
	if !ns.vl.TryAcquire(lockKey) {
		return nil, status.Errorf(codes.Aborted, volumeOperationAlreadyExistsFmt, id)
	}
	defer ns.vl.Release(lockKey)

	isMountPoint, err := ns.mounter.IsMountPoint(targetPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if isMountPoint {
		if err := mount.CleanupMountPoint(targetPath, ns.mounter, true); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}
