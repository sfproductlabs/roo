package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/mitchellh/mapstructure"
)

func GetDockerRoutes() ([]Route, error) {

	/////////////// CONNECT TO SOCKET
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("Couldn't connect to docker socket")
	}
	taskFilter := filters.NewArgs()
	tasks, err := cli.TaskList(ctx, types.TaskListOptions{Filters: taskFilter})

	/////////////////////// SCAN TASKS
	var conz []Route
	for _, task := range tasks {
		if label, ok := task.Spec.ContainerSpec.Labels["OriginHost"]; ok {
			var route Route
			err := mapstructure.Decode(task.Spec.ContainerSpec.Labels, &route)
			if err != nil {
				rlog.Infof("Error loading route for task %s (%s), label %s", task.Name, task.ID, label)
				continue
			}
			conz = append(conz, route)
		}
	}
	return conz, nil
}
