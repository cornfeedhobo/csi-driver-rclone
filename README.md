# csi-driver-rclone

A CSI driver using rclone.

## Architecture

### Node Plugin

The Node plugin is a gRPC server that needs to run on the Node where the volume
will be provisioned.  So suppose you have a Kubernetes cluster with three nodes
where your Pod's are scheduled, you would deploy this to all three nodes.

### Controller Plugin

The Controller plugin is a gRPC server that can run anywhere.  In terms of a
Kubernetes cluster, it can run on any node (even on the master node).

## Is it any good

[Yes](http://news.ycombinator.com/item?id=3067434)
