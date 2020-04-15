//package eyedoc //thanks @psytron
package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	client "github.com/docker/docker/client"
	//"time"
)

func GetDockerTasks() []map[string]string {
	/////////////// CONNECT TO SOCKET
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	taskFilter := filters.NewArgs()
	tasks, err := cli.TaskList(ctx, types.TaskListOptions{Filters: taskFilter})

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
	return conz
}
