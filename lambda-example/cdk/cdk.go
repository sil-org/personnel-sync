package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type PersonnelSyncStackProps struct {
	awscdk.StackProps
}

func NewPersonnelSyncStack(scope constructs.Construct, id string, props *PersonnelSyncStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}

	stack := awscdk.NewStack(scope, &id, &sprops)

	logGroup := awslogs.NewLogGroup(stack, jsii.String("LambdaLogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.Sprintf("/aws/lambda/%s-cdk", id),
		Retention:     awslogs.RetentionDays_TWO_MONTHS,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	fn := awslambda.NewFunction(stack, &id, &awslambda.FunctionProps{
		Runtime:       awslambda.Runtime_PROVIDED_AL2023(),
		Handler:       jsii.String("bootstrap"),
		Code:          awslambda.Code_FromAsset(jsii.String("../src/bin"), nil),
		FunctionName:  &id,
		Timeout:       awscdk.Duration_Seconds(jsii.Number(900)),
		MemorySize:    jsii.Number(128),
		LogGroup:      logGroup,
		LoggingFormat: awslambda.LoggingFormat_JSON,
	})

	fn.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings(
			"ses:SendEmail",
		),
		Resources: jsii.Strings("*"),
	}))

	schedules := map[string]awsevents.Schedule{
		"example-sync": awsevents.Schedule_Cron(&awsevents.CronOptions{
			Minute: jsii.String("0"),
			Hour:   jsii.String("0"),
		}),
	}

	for name, schedule := range schedules {
		rule := awsevents.NewRule(stack, jsii.String(name), &awsevents.RuleProps{Schedule: schedule})

		rule.AddTarget(awseventstargets.NewLambdaFunction(fn, &awseventstargets.LambdaFunctionProps{
			Event: awsevents.RuleTargetInput_FromObject(&map[string]*string{
				"ConfigPath": jsii.Sprintf("%s.json", name),
			}),
			RetryAttempts: jsii.Number(0),
		}))
	}

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	const name = "personnel-sync-example"

	NewPersonnelSyncStack(app, name, &PersonnelSyncStackProps{
		awscdk.StackProps{
			Env: &awscdk.Environment{
				Region: jsii.String("us-east-1"),
			},
			Tags: &map[string]*string{
				"managed_by":        jsii.String("cdk"),
				"itse_app_name":     jsii.String(name),
				"itse_app_customer": jsii.String("gtis"),
				"itse_app_env":      jsii.String("production"),
			},
		},
	})

	app.Synth(nil)
}
