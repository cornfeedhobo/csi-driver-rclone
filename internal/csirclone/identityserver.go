package csirclone

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IdentityServer consists of basic methods, mainly for identifying the service,
// making sure it's healthy, and returning basic information about the plugin itself.
type IdentityServer struct {
	d *Driver
}

// NewIdentityServer satisfies the csi.IdentityServer interface.
func NewIdentityServer(d *Driver) *IdentityServer {
	return &IdentityServer{d}
}

// GetPluginInfo returns the name and version of the plugin.
func (ids *IdentityServer) GetPluginInfo(_ context.Context, _ *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	if ids.d.DriverName == "" {
		return nil, status.Error(codes.Unavailable, "Driver name not configured")
	}

	if ids.d.Version == "" {
		return nil, status.Error(codes.Unavailable, "Driver is missing version")
	}

	return &csi.GetPluginInfoResponse{
		Name:          ids.d.DriverName,
		VendorVersion: ids.d.Version,
	}, nil
}

// Probe checks whether the plugin is running or not.
// This method does not need to return anything.
// Currently the spec does not dictate what you should return either.
// Hence, return an empty responsDrivere
func (ids *IdentityServer) Probe(_ context.Context, _ *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}

// GetPluginCapabilities returns the capabilities of the plugin.
// Currently it reports whether the plugin has the ability of serving the
// Controller interface. The CO calls the Controller interface methods
// depending on whether this method returns the capability or not.
func (ids *IdentityServer) GetPluginCapabilities(_ context.Context, _ *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}
