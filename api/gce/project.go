package gce

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/resourcemanager/apiv3"
	rmpb "google.golang.org/genproto/googleapis/cloud/resourcemanager/v3"
)

func GetProjectNumber(projectId string) (string, error) {
	ctx := context.Background()

	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	req := &rmpb.GetProjectRequest{
		Name: fmt.Sprintf("projects/%s", projectId),
	}
	project, err := client.GetProject(ctx, req)
	if err != nil {
		return "", err
	}

	result := strings.Split(project.Name, "/")
	return fmt.Sprintf("%v", result[1]), nil
}
