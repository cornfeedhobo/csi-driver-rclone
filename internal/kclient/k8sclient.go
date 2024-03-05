package kclient

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Config clientcmd.ClientConfig
	Set    kubernetes.Interface
}

func NewClient() (*Client, error) {

	c := &Client{
		Config: clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		),
	}

	rc, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	c.Set, err = kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	return c, nil

}

func (c *Client) GetSecret(name string) (*corev1.Secret, error) {
	namespace, _, err := c.Config.Namespace()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting current namespace '%s', %s", name, err)
	}

	secret, err := c.Set.CoreV1().
		Secrets(namespace).
		Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting secret '%s', %s", name, err)
	}

	return secret, nil
}
