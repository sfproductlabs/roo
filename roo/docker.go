package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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
	if err != nil {
		return nil, fmt.Errorf("[WARNING] Error loading swarm tasks\n")
	}

	/////////////////////// SCAN TASKS
	var conz []Route
	for _, task := range tasks {
		//TODO - consider multiple hosts
		hosts := strings.Split(task.Spec.ContainerSpec.Labels["OriginHosts"], ",")
		hosts = append(hosts, task.Spec.ContainerSpec.Labels["OriginHost"])
		for _, host := range hosts {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			route := Route{
				OriginHost:        host,
				OriginPort:        task.Spec.ContainerSpec.Labels["OriginPort"],
				OriginScheme:      task.Spec.ContainerSpec.Labels["OriginScheme"],
				DestinationHost:   task.Spec.ContainerSpec.Labels["DestinationHost"],
				DestinationPort:   task.Spec.ContainerSpec.Labels["DestinationPort"],
				DestinationScheme: task.Spec.ContainerSpec.Labels["DestinationScheme"],
			}
			if route.OriginHost == "" || route.DestinationHost == "" || route.DestinationPort == "" {
				rlog.Infof("Error loading route for task %s (%s), route %s", task.Name, task.ID, route)
				continue
			}
			conz = append(conz, route)
		}

	}
	return conz, nil
}
