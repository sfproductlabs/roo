package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/mitchellh/mapstructure"
)

type Route struct {
	OriginScheme      string
	OriginHost        string
	OriginPort        string
	DestinationScheme string
	DestinationHost   string
	DestinationPort   string
}

func Tasks() []Route {

	/////////////// CONNECT TO SOCKET
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	taskFilter := filters.NewArgs()
	tasks, err := cli.TaskList(ctx, types.TaskListOptions{Filters: taskFilter})

	/////////////////////// SCAN TASKS
	var conz []Route
	for _, task := range tasks {
		if val, ok := task.Spec.ContainerSpec.Labels["OriginHost"]; ok {
			var route Route
			//route.DestinationHost = "sometihng"
			err := mapstructure.Decode(task.Spec.ContainerSpec.Labels, &route)
			if err != nil {
				panic(err)
			}
			fmt.Println(route, val)
			conz = append(conz, route)
		}
	}
	return conz
}
