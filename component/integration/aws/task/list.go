package task

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/turnerlabs/udeploy/component/version"
	"github.com/turnerlabs/udeploy/model"
)

func List(instance model.Instance, svc *ecs.ECS, status *string) (tasks []*ecs.Task, err error) {
	output, err := svc.ListTasks(&ecs.ListTasksInput{
		Cluster:       &instance.Cluster,
		Family:        aws.String(instance.Task.Family),
		DesiredStatus: status,
	})
	if err != nil {
		return make([]*ecs.Task, 0), err
	}
	if len(output.TaskArns) == 0 {
		return make([]*ecs.Task, 0), nil
	}
	tasksOutput, err := svc.DescribeTasks(&ecs.DescribeTasksInput{
		Cluster: &instance.Cluster,
		Tasks:   output.TaskArns,
	})
	if err != nil {
		return make([]*ecs.Task, 0), err
	}
	return tasksOutput.Tasks, nil
}

func ListDefinitions(taskDefinition model.Task) (map[string]model.Definition, error) {

	arns := []*string{}

	svc := ecs.New(session.New())
	arns, err := listTaskDefinitionArns(svc, taskDefinition.Family, "", arns)
	if err != nil {
		return nil, err
	}

	tds, err := getTaskDefinitions(svc, arns, taskDefinition.Revisions)
	if err != nil {
		return nil, err
	}
	return keepMostRecentRevisions(tds, taskDefinition.ImageTagEx), nil
}

func GetTasksInfo(instance model.Instance, svc *ecs.ECS) ([]model.TaskInfo, error) {
	runningTasks, err := List(instance, svc, aws.String("RUNNING"))
	if err != nil {
		return []model.TaskInfo{}, err
	}
	stoppedTasks, err := List(instance, svc, aws.String("STOPPED"))
	if err != nil {
		return []model.TaskInfo{}, err
	}
	tasks := append(runningTasks, stoppedTasks...)
	if len(tasks) == 0 {
		return []model.TaskInfo{}, nil
	}

	tasksInfo := make([]model.TaskInfo, 0)
	for _, task := range tasks {
		for _, container := range task.Containers {

			parts := strings.Split(*task.TaskArn, "/")

			taskID := parts[len(parts)-1]

			o, err := svc.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
				TaskDefinition: task.TaskDefinitionArn,
			})
			if err != nil {
				return []model.TaskInfo{}, err
			}

			logLink := getLogLink(*o.TaskDefinition.ContainerDefinitions[0].LogConfiguration, taskID, *container.Name)

			taskInfo := model.TaskInfo{
				TaskID:     taskID,
				LastStatus: *task.LastStatus,
				Version:    version.FormatExtract(*o.TaskDefinition.ContainerDefinitions[0].Image, instance.Task.ImageTagEx),
				LogLink:    logLink,
			}
			if *task.LastStatus == "STOPPED" && task.StoppedReason != nil {
				taskInfo.LastStatusTime = *task.StoppedAt
				taskInfo.Reason = *task.StoppedReason
			} else {
				if task.StartedAt != nil {
					taskInfo.LastStatusTime = *task.StartedAt
				}

				taskInfo.Reason = fmt.Sprintf("Started by %s", *task.StartedBy)
			}
			tasksInfo = append(tasksInfo, taskInfo)
		}
	}
	return tasksInfo, nil
}

func getLogLink(logConfig ecs.LogConfiguration, taskID string, containerName string) string {
	logGroup := logConfig.Options["awslogs-group"]
	logRegion := logConfig.Options["awslogs-region"]
	logStreamPrefix := logConfig.Options["awslogs-stream-prefix"]

	logLink := fmt.Sprintf("https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#logEventViewer:group=%s;stream=%s/%s/%s",
		*logRegion, *logRegion, *logGroup, *logStreamPrefix, containerName, taskID)

	return logLink
}

func keepMostRecentRevisions(tds []*ecs.TaskDefinition, regex string) map[string]model.Definition {
	releases := map[string]model.Definition{}

	for _, td := range tds {
		release := model.DefinitionFrom(td, regex)
		ver := release.FormatVersion()

		if ver == "undetermined" {
			continue
		}

		if len(ver) > 1 {
			if tdv, found := releases[ver]; found {
				if *td.Revision > tdv.Revision {
					releases[ver] = model.DefinitionFrom(td, regex)
				}
			} else {
				releases[ver] = model.DefinitionFrom(td, regex)
			}
		}
	}

	return releases
}

func getTaskDefinitions(svc *ecs.ECS, arns []*string, maxRegistryEntries int) ([]*ecs.TaskDefinition, error) {
	tds := []*ecs.TaskDefinition{}

	count := 0
	for _, i := range arns {
		count++

		o, err := svc.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
			TaskDefinition: i,
		})

		if err != nil {
			return nil, err
		}

		tds = append(tds, o.TaskDefinition)

		if count >= maxRegistryEntries {
			break
		}
	}

	return tds, nil
}

func listTaskDefinitionArns(svc *ecs.ECS, prefix, nextToken string, arns []*string) ([]*string, error) {

	input := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: aws.String(prefix),
		Sort:         aws.String("DESC"),
	}

	if len(nextToken) > 0 {
		input.SetNextToken(nextToken)
	}

	output, err := svc.ListTaskDefinitions(input)
	if err != nil {
		return nil, err
	}

	arns = append(arns, output.TaskDefinitionArns...)

	if output.NextToken == nil || len(*output.NextToken) == 0 {
		return arns, nil
	}

	return listTaskDefinitionArns(svc, prefix, *output.NextToken, arns)
}
