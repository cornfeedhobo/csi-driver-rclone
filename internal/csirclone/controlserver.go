package csirclone

import (
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cornfeedhobo/csi-driver-rclone/internal/csicommon"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// ControlServer is responsible of controlling and managing the volumes,
// such as: creating, deleting, attaching/detaching, snapshotting, etc..
type ControlServer struct {
	*csicommon.ControlServer

	driver *Driver
	caps   []*csi.ControllerServiceCapability
}

// NewControlServer returns a working control server.
func NewControlServer(d *Driver) *ControlServer {

	cs := &ControlServer{
		ControlServer: &csicommon.ControlServer{},
		driver:        d,
	}

	cs.SetCapabilities([]csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	})

	return cs

}

// SetCapabilities
func (cs *ControlServer) SetCapabilities(caps []csi.ControllerServiceCapability_RPC_Type) {
	cs.caps = make([]*csi.ControllerServiceCapability, len(caps))
	for idx, cap := range caps {
		klog.Infof("Enabling Controller Capability %s", cap)
		cs.caps[idx] = &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}
}

// ControllerExpandVolume
func (cs *ControlServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {

	id := req.GetVolumeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	exist, err := cs.driver.IsVolume(ctx, id)
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

	if err := cs.driver.ExpandVolume(ctx, id, capacity.RequiredBytes); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, status.Error(codes.NotFound, "specified volume does not exist")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes: capacity.RequiredBytes,
	}, nil
}

// ControllerGetCapabilities
func (cs *ControlServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.caps,
	}, nil
}

// CreateVolume
func (cs *ControlServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {

	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	klog.V(2).Infof("CreateVolume: name: %s", name)

	err := cs.validateVolumeCapabilities(req.GetVolumeCapabilities())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	parameters := req.GetParameters()
	if parameters == nil {
		parameters = make(map[string]string)
	}
	klog.V(2).Infof("CreateVolume: parameters: %v", parameters)

	newVolume := NewVolume(
		cs.driver.Remote,
		name,
		req.CapacityRange.RequiredBytes,
	)

	klog.V(2).Infof("CreateVolume: checking if volume '%s' already exists", newVolume.ID)

	curVolume, err := cs.driver.ReadVolume(ctx, newVolume.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "CreateVolume: %s", err)
	}

	if curVolume != nil {
		klog.V(2).Infof("CreateVolume: volume already exists, validating metadata")

		err = curVolume.IsConflict(newVolume)
		if err != nil {
			// special case, required to pass csi-sanity
			if errors.Is(err, ErrMetaWrongCapacity) {
				return nil, status.Error(codes.AlreadyExists, err.Error())
			}
			return nil, err // FIXMEs
		}

		klog.V(2).Infof("CreateVolume: volume already exists and is healthy")

		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:      curVolume.ID,
				CapacityBytes: curVolume.Capacity,
				VolumeContext: parameters,
				ContentSource: req.GetVolumeContentSource(),
			},
		}, nil
	}

	klog.V(2).Info("CreateVolume: volume does not exist, creating")

	err = cs.driver.WriteVolume(ctx, newVolume)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      newVolume.ID,
			CapacityBytes: 0, // by setting it to zero, Provisioner will use PVC requested size as PV size
			VolumeContext: parameters,
			ContentSource: req.GetVolumeContentSource(),
		},
	}, nil
}

// DeleteVolume
func (cs *ControlServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {

	id := req.GetVolumeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	exist, err := cs.driver.IsVolume(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if exist {
		err = cs.driver.PurgeVolume(ctx, id)
	}

	return &csi.DeleteVolumeResponse{}, err
}

// ValidateVolumeCapabilities
func (cs *ControlServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {

	id := req.GetVolumeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	exist, err := cs.driver.IsVolume(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	if !exist {
		return nil, status.Errorf(codes.NotFound, "Volume with ID '%s' does not exist", id)
	}

	if err := cs.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid capabilities: %s", err)
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
		Message: "",
	}, nil
}

// validateVolumeCapabilities validates the given VolumeCapability array is valid
func (cs *ControlServer) validateVolumeCapabilities(caps []*csi.VolumeCapability) error {

	if len(caps) == 0 {
		return status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}

	for _, c := range caps {
		if c.GetBlock() != nil {
			return status.Error(codes.InvalidArgument, "block volume capability not supported")
		}
	}

	return nil
}
