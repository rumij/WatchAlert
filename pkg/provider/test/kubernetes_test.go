package test

import (
	"context"
	"fmt"
	"testing"
	"watchAlert/pkg/provider"

	"github.com/sirupsen/logrus"
)

func TestKubernetesClient(t *testing.T) {
	cli, err := provider.NewKubernetesClient(context.Background(), "", nil)
	if err != nil {
		logrus.Error(err)
		return
	}

	event, err := cli.GetWarningEvent("", 10, []string{})
	if err != nil {
		logrus.Error(err)
		return
	}

	fmt.Println(event)
}
