//package eyedoc //thanks @psytron
package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	//"time"
)

//This should pass the full label string of the filtered routes from all the labels
//It should be an array of the form
//["com.roo.host:tr.sfpl.io:https=http://tracker_tracker:8443", etc....]
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
		return nil, err
	}
	fmt.Println("DEBUG", tasks)

	/////////////////////// SCAN TASKS
	var conz []map[string]string
	for _, task := range tasks {
		if val, ok := task.Spec.ContainerSpec.Labels[HOST_PREFIX]; ok {
			for _, ntrk := range task.NetworksAttachments {
				fmt.Println("DEBUG", task, val)
				ob := map[string]string{}
				ob["container"] = task.ID
				ob["com.roo.host"] = val
				ob["receive"] = ntrk.Addresses[0]
				conz = append(conz, ob)
			}
		}
	}
	return nil, nil
}
